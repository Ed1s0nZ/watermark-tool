package main

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"github.com/gin-gonic/gin"
	"io"
	"mime/multipart"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"watermark-tool/internal/auth"
	"watermark-tool/internal/distribution"
)

func TestPlatformHTTP(t *testing.T) {
	clientRequestsMu.Lock()
	clientRequestsMap = make(map[string]*ClientRequests)
	clientRequestsMu.Unlock()
	store, err := distribution.Open(filepath.Join(t.TempDir(), "data"), bytes.Repeat([]byte{42}, 32), bytes.Repeat([]byte{7}, 32))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	r := gin.New()
	session := auth.New("admin", "test-password-long", false)
	r.POST("/login", session.Login)
	api := r.Group("/api", auth.SameOrigin(), session.Require())
	registerPlatform(api, store, t.TempDir())
	login := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/login", strings.NewReader(`{"username":"admin","password":"test-password-long"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(login, req)
	cookie := login.Result().Cookies()[0]
	requestJSON := func(path, body string, authenticated bool) *httptest.ResponseRecorder {
		req := httptest.NewRequest("POST", path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		if authenticated {
			req.AddCookie(cookie)
		}
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w
	}
	if w := requestJSON("/api/platform/records/search", `{}`, false); w.Code != 401 {
		t.Fatal(w.Code)
	}
	source := imageFixture()
	send := func(path, csv, key string, data []byte) *httptest.ResponseRecorder {
		var body bytes.Buffer
		form := multipart.NewWriter(&body)
		form.WriteField("recipients_csv", csv)
		part, _ := form.CreateFormFile("file", "original.png")
		part.Write(data)
		form.Close()
		req := httptest.NewRequest("POST", path, &body)
		req.Header.Set("Content-Type", form.FormDataContentType())
		req.Header.Set("Idempotency-Key", key)
		req.AddCookie(cookie)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w
	}
	csv := "recipient_id,name\n001,Alice\n002,Bob\n003,名字"
	generated := send("/api/platform/batches", csv, "http-request-00000001", source)
	if generated.Code != 200 {
		t.Fatal(generated.Code, generated.Body.String())
	}
	batchID := generated.Header().Get("X-Batch-ID")
	if batchID == "" {
		t.Fatal("missing ID")
	}
	archive, err := zip.NewReader(bytes.NewReader(generated.Body.Bytes()), int64(generated.Body.Len()))
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	var first []byte
	for _, f := range archive.File {
		if !strings.HasSuffix(f.Name, ".png") {
			continue
		}
		file, _ := f.Open()
		data, _ := io.ReadAll(file)
		file.Close()
		if first == nil {
			first = data
		}
		traced := send("/api/platform/trace", "", "", data)
		if traced.Code != 200 {
			t.Fatal(traced.Code, traced.Body.String())
		}
		var result struct {
			Match  string              `json:"match"`
			Record distribution.Record `json:"record"`
		}
		json.Unmarshal(traced.Body.Bytes(), &result)
		if result.Match != "registered" || result.Record.BatchID != batchID || seen[result.Record.ID] {
			t.Fatal(result)
		}
		seen[result.Record.ID] = true
	}
	if len(seen) != 3 {
		t.Fatal(seen)
	}
	retry := send("/api/platform/batches", csv, "http-request-00000001", source)
	if retry.Code != 200 || retry.Header().Get("X-Batch-ID") != batchID || !bytes.Equal(generated.Body.Bytes(), retry.Body.Bytes()) {
		t.Fatal("idempotent ZIP changed", retry.Code, retry.Body.String())
	}
	if w := send("/api/platform/batches", csv+"\n004,Other", "http-request-00000001", source); w.Code != 409 {
		t.Fatal(w.Code)
	}
	rebuilt := send("/api/platform/batches/"+batchID+"/download", "", "", source)
	if rebuilt.Code != 200 || !bytes.Equal(rebuilt.Body.Bytes(), generated.Body.Bytes()) {
		t.Fatal("rebuild mismatch", rebuilt.Code)
	}
	result := requestJSON("/api/platform/records/search", `{"q":"名字","page":1,"page_size":1}`, true)
	if result.Code != 200 || !strings.Contains(result.Body.String(), "名字") {
		t.Fatal(result.Code, result.Body.String())
	}
	created := requestJSON("/api/platform/keys", "", true)
	if created.Code != 201 || strings.Contains(created.Body.String(), "secret") {
		t.Fatal(created.Code, created.Body.String())
	}
	var key distribution.Key
	json.Unmarshal(created.Body.Bytes(), &key)
	if w := requestJSON("/api/platform/keys/"+key.ID+"/state", `{"state":"active"}`, true); w.Code != 200 {
		t.Fatal(w.Code)
	}
	if w := send("/api/platform/batches", "bad", "http-request-00000002", source); w.Code != 400 {
		t.Fatal(w.Code)
	}
	_ = first
}
