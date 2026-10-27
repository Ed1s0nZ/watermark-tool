package main

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/gin-gonic/gin"
	"image/png"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"watermark-tool/internal/auth"
	"watermark-tool/internal/blind"
	"watermark-tool/internal/distribution"
	"watermark-tool/internal/quality"
)

type cancellingWriter struct {
	*httptest.ResponseRecorder
	cancel context.CancelFunc
}

func (w cancellingWriter) Flush() {
	w.ResponseRecorder.Flush()
	if strings.Contains(w.Body.String(), `"type":"item"`) {
		w.cancel()
	}
}
func TestTraceBatchAndDualQualityHTTP(t *testing.T) {
	clientRequestsMu.Lock()
	clientRequestsMap = map[string]*ClientRequests{}
	clientRequestsMu.Unlock()
	store, e := distribution.Open(filepath.Join(t.TempDir(), "data"), bytes.Repeat([]byte{42}, 32), bytes.Repeat([]byte{7}, 32))
	if e != nil {
		t.Fatal(e)
	}
	defer store.Close()
	r := gin.New()
	session := auth.New("admin", "long-test-password", false)
	r.POST("/login", session.Login)
	api := r.Group("/api", auth.SameOrigin(), session.Require(), auditMiddleware(store))
	temp := t.TempDir()
	registerBatchTrace(api, store, temp)
	registerWorkbench(api, store, temp)
	login := httptest.NewRecorder()
	request := httptest.NewRequest("POST", "/login", strings.NewReader(`{"username":"admin","password":"long-test-password"}`))
	r.ServeHTTP(login, request)
	cookie := login.Result().Cookies()[0]
	original := imageFixture()
	img, _ := png.Decode(bytes.NewReader(original))
	b, key, e := store.Begin("trace-register-request", distribution.Digest(original), "admin", []distribution.Recipient{{ID: "1", Name: "SensitiveRecipient"}})
	if e != nil {
		t.Fatal(e)
	}
	marked, e := blind.Embed(img, b.Copies[0].ID, key, b.CreatedAt)
	if e != nil {
		t.Fatal(e)
	}
	var reg bytes.Buffer
	png.Encode(&reg, marked)
	if e = store.Finish(b.ID, ""); e != nil {
		t.Fatal(e)
	}
	foreign, _ := blind.Embed(img, "not-registered", key, time.Now())
	var unreg bytes.Buffer
	png.Encode(&unreg, foreign)
	makeRequest := func(path string, inputs [][]byte, policy string, logged bool) *http.Request {
		var body bytes.Buffer
		form := multipart.NewWriter(&body)
		field := "files"
		if path == "dual" {
			field = "file"
			form.WriteField("watermark", "quality-text")
			form.WriteField("quality_policy", policy)
			part, _ := form.CreateFormFile("original", "raw.png")
			part.Write(original)
		}
		for _, data := range inputs {
			part, _ := form.CreateFormFile(field, "=SensitiveFilename<script>.png")
			part.Write(data)
		}
		form.Close()
		req := httptest.NewRequest("POST", "/api/workbench/"+path, &body)
		req.Header.Set("Content-Type", form.FormDataContentType())
		if logged {
			req.AddCookie(cookie)
		}
		return req
	}
	send := func(req *http.Request) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w
	}
	if w := send(makeRequest("trace-batch", [][]byte{original}, "", false)); w.Code != 401 {
		t.Fatal(w.Code)
	}
	cross := makeRequest("trace-batch", [][]byte{original}, "", true)
	cross.Header.Set("Origin", "https://untrusted.invalid")
	if w := send(cross); w.Code != 403 {
		t.Fatal(w.Code)
	}
	many := make([][]byte, 21)
	for i := range many {
		many[i] = original
	}
	if w := send(makeRequest("trace-batch", many, "", true)); w.Code != 422 {
		t.Fatal(w.Code)
	}
	w := send(makeRequest("trace-batch", [][]byte{reg.Bytes(), unreg.Bytes(), original, []byte("broken"), original}, "", true))
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	lines := strings.Split(strings.TrimSpace(w.Body.String()), "\n")
	if len(lines) != 7 {
		t.Fatal(w.Body.String())
	}
	want := []string{"registered", "unregistered", "not_found", "error", "not_found"}
	for i, status := range want {
		var row struct {
			Report    verificationReport `json:"report"`
			Duplicate int                `json:"duplicate_of"`
		}
		if json.Unmarshal([]byte(lines[i+1]), &row) != nil || row.Report.Status != status || row.Report.Source == "" {
			t.Fatal(lines[i+1])
		}
		if i == 0 && (row.Report.Record == nil || row.Report.Record.Copy.ID != b.Copies[0].ID) {
			t.Fatal("registration")
		}
		if i == 4 && row.Duplicate != 3 {
			t.Fatal("duplicate")
		}
	}
	if !strings.Contains(lines[6], `"status":"partial"`) {
		t.Fatal(lines[6])
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	req := makeRequest("trace-batch", [][]byte{original, original, original}, "", true).WithContext(ctx)
	cw := cancellingWriter{httptest.NewRecorder(), cancel}
	r.ServeHTTP(cw, req)
	if strings.Count(cw.Body.String(), `"type":"item"`) != 1 || strings.Contains(cw.Body.String(), `"type":"end"`) {
		t.Fatal(cw.Body.String())
	}
	if len(blindSlots) != 0 {
		t.Fatal("leaked worker")
	}
	for _, policy := range []string{`{"minimum_psnr_db":19}`, `{"unknown":35}`} {
		if w = send(makeRequest("dual", [][]byte{original}, policy, true)); w.Code != 422 {
			t.Fatal(w.Code)
		}
	}
	w = send(makeRequest("dual", [][]byte{original}, `{}`, true))
	var q quality.Summary
	if w.Code != 200 || json.Unmarshal([]byte(w.Header().Get("X-Watermark-Quality")), &q) != nil || !q.Accepted || q.Visible == nil || q.Visible.Status != "infinite" {
		t.Fatal(w.Code, w.Body.String(), q)
	}
	w = send(makeRequest("dual", [][]byte{original}, `{"minimum_psnr_db":60}`, true))
	var failed struct {
		Code    string          `json:"code"`
		Quality quality.Summary `json:"quality"`
	}
	json.Unmarshal(w.Body.Bytes(), &failed)
	if w.Code != 422 || failed.Code != "quality_failed" || failed.Quality.Accepted || !failed.Quality.Verified {
		t.Fatal(w.Code, w.Body.String())
	}
	rows, _, e := store.AuditSearch(distribution.AuditFilter{Page: 1, Size: 100}, false)
	if e != nil {
		t.Fatal(e)
	}
	raw, _ := json.Marshal(rows)
	if bytes.Contains(raw, []byte("SensitiveFilename")) || bytes.Contains(raw, []byte("SensitiveRecipient")) || bytes.Contains(raw, []byte("quality-text")) {
		t.Fatal("private audit", string(raw))
	}
	hasCancel := false
	for _, a := range rows {
		if a.Action == "trace.batch" && a.Outcome == "cancelled" {
			hasCancel = true
		}
	}
	if !hasCancel {
		t.Fatal("cancel audit")
	}
	// Request temporary files and processing resources are gone; another batch can run.
	entries, _ := os.ReadDir(temp)
	if len(entries) != 0 {
		t.Fatal("temp leak", entries)
	}
}
