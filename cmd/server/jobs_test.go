package main

import (
	"archive/zip"
	"bytes"
	"encoding/csv"
	"encoding/json"
	"github.com/gin-gonic/gin"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"mime/multipart"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"watermark-tool/internal/auth"
	"watermark-tool/internal/delivery"
	"watermark-tool/internal/distribution"
)

func TestJobsAndAuditHTTP(t *testing.T) {
	clientRequestsMu.Lock()
	clientRequestsMap = map[string]*ClientRequests{}
	clientRequestsMu.Unlock()
	store, err := distribution.Open(filepath.Join(t.TempDir(), "data"), bytes.Repeat([]byte{42}, 32), bytes.Repeat([]byte{7}, 32))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	tasks, err := delivery.New(store, blindSlots)
	if err != nil {
		t.Fatal(err)
	}
	defer tasks.Close()
	r := gin.New()
	sessions := auth.New("admin", "test-password-long", false)
	authAPI := r.Group("/api/auth", auth.SameOrigin(), auditMiddleware(store))
	authAPI.POST("/login", sessions.Login)
	api := r.Group("/api", auth.SameOrigin(), sessions.Require(), auditMiddleware(store))
	registerJobs(api, store, tasks, t.TempDir())
	registerAudit(api, store, tasks)
	login := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/auth/login", strings.NewReader(`{"username":"admin","password":"test-password-long"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(login, req)
	cookie := login.Result().Cookies()[0]
	sendJSON := func(method, path string, value any, authenticated bool) *httptest.ResponseRecorder {
		var body bytes.Buffer
		if value != nil {
			json.NewEncoder(&body).Encode(value)
		}
		req := httptest.NewRequest(method, path, &body)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Idempotency-Key", "http-matrix-request-0001")
		if authenticated {
			req.AddCookie(cookie)
		}
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w
	}
	if w := sendJSON("GET", "/api/audit/status", nil, false); w.Code != 401 {
		t.Fatal(w.Code)
	}
	first := imageFixture()
	img, _, _ := image.Decode(bytes.NewReader(first))
	second := image.NewNRGBA(img.Bounds())
	draw.Draw(second, second.Bounds(), img, img.Bounds().Min, draw.Src)
	second.Set(20, 20, color.NRGBA{10, 220, 40, 255})
	var buffer bytes.Buffer
	png.Encode(&buffer, second)
	data := [][]byte{first, buffer.Bytes()}
	sources := []distribution.JobSource{{Name: "one.png", SHA256: distribution.Digest(data[0]), Width: 512, Height: 512}, {Name: "two.png", SHA256: distribution.Digest(data[1]), Width: 512, Height: 512}}
	input := map[string]any{"sources": sources, "recipients_csv": "recipient_id,name\n001,SensitiveRecipient-A\n002,接收人B", "mode": "blind"}
	created := sendJSON("POST", "/api/jobs", input, true)
	var job distribution.Job
	if err = json.Unmarshal(created.Body.Bytes(), &job); err != nil || created.Code != 201 || len(job.Copies) != 4 || job.RequestHash != "" {
		t.Fatal(created.Code, created.Body.String())
	}
	repeated := sendJSON("POST", "/api/jobs", input, true)
	var same distribution.Job
	json.Unmarshal(repeated.Body.Bytes(), &same)
	if same.ID != job.ID {
		t.Fatal("duplicate job")
	}
	if w := sendJSON("POST", "/api/jobs/"+job.ID+"/start", nil, true); w.Code != 409 {
		t.Fatal(w.Code)
	}
	for i, src := range job.Sources {
		var body bytes.Buffer
		form := multipart.NewWriter(&body)
		form.WriteField("source_id", src.ID)
		part, _ := form.CreateFormFile("file", "source.png")
		part.Write(data[i])
		form.Close()
		req := httptest.NewRequest("POST", "/api/jobs/"+job.ID+"/inputs", &body)
		req.Header.Set("Content-Type", form.FormDataContentType())
		req.Header.Set("X-Upload-Token", job.UploadToken)
		req.AddCookie(cookie)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != 200 {
			t.Fatal(w.Code, w.Body.String())
		}
	}
	if w := sendJSON("POST", "/api/jobs/"+job.ID+"/start", nil, true); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	until := time.Now().Add(30 * time.Second)
	for time.Now().Before(until) {
		w := sendJSON("GET", "/api/jobs/"+job.ID, nil, true)
		if w.Code != 200 {
			t.Fatal(w.Code)
		}
		json.Unmarshal(w.Body.Bytes(), &job)
		if job.Status == "completed" {
			break
		}
		if job.Status == "failed" {
			t.Fatal(job.Error)
		}
		time.Sleep(20 * time.Millisecond)
	}
	if job.Status != "completed" || job.DownloadState != "ready" {
		t.Fatal(job.Status, job.Error)
	}
	downloaded := sendJSON("GET", "/api/jobs/"+job.ID+"/archive", nil, true)
	if downloaded.Code != 200 || distribution.Digest(downloaded.Body.Bytes()) != job.ArchiveSHA256 {
		t.Fatal(downloaded.Code, downloaded.Body.String())
	}
	archive, err := zip.NewReader(bytes.NewReader(downloaded.Body.Bytes()), int64(downloaded.Body.Len()))
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, file := range archive.File {
		if !strings.HasSuffix(file.Name, ".png") {
			continue
		}
		reader, _ := file.Open()
		img, _, e := image.Decode(reader)
		reader.Close()
		if e != nil {
			t.Fatal(e)
		}
		result, keyID, e := store.Verify(img, "")
		if e != nil {
			t.Fatal(e)
		}
		record, e := store.Lookup(result, keyID)
		if e != nil || record == nil || record.JobID != job.ID {
			t.Fatal(record, e)
		}
		count++
	}
	if count != 4 {
		t.Fatal(count)
	}
	var audit struct {
		Items []distribution.AuditEvent `json:"items"`
		Total int                       `json:"total"`
	}
	queried := sendJSON("POST", "/api/audit/search", map[string]any{"action": "task.create", "page": 1, "page_size": 20}, true)
	json.Unmarshal(queried.Body.Bytes(), &audit)
	if queried.Code != 200 || audit.Total != 1 || audit.Items[0].Target != job.ID || audit.Items[0].Actor != "admin" {
		t.Fatal(queried.Code, queried.Body.String())
	}
	all := sendJSON("POST", "/api/audit/export", map[string]any{"format": "json"}, true)
	if all.Code != 200 || strings.Contains(all.Body.String(), "SensitiveRecipient") || strings.Contains(all.Body.String(), "test-password-long") || strings.Contains(all.Body.String(), cookie.Value) {
		t.Fatal("audit leaked input", all.Body.String())
	}
	store.Audit("=SUM(1,2)", "test.export", "", "completed", "")
	exported := sendJSON("POST", "/api/audit/export", map[string]any{"actor": "=SUM(1,2)", "format": "csv"}, true)
	if exported.Code != 200 {
		t.Fatal(exported.Code)
	}
	rows, e := csv.NewReader(strings.NewReader(exported.Body.String())).ReadAll()
	if e != nil || len(rows) != 2 || rows[1][3] != "'=SUM(1,2)" {
		t.Fatal(rows, e)
	}
	observed, total, e := store.AuditSearch(distribution.AuditFilter{Action: "task.download", Page: 1, Size: 20}, false)
	if e != nil || total != 2 || observed[0].Outcome != "response_sent" {
		t.Fatal(observed, e)
	}
	req = httptest.NewRequest("POST", "/api/jobs", strings.NewReader(`{}`))
	req.AddCookie(cookie)
	req.Header.Set("Origin", "https://other.invalid")
	denied := httptest.NewRecorder()
	r.ServeHTTP(denied, req)
	if denied.Code != 403 {
		t.Fatal(denied.Code)
	}
}
func TestJobRetentionOptions(t *testing.T) {
	for _, value := range []string{"0", "3601", "bad"} {
		t.Setenv("WATERMARK_JOB_ARCHIVE_TTL_SECONDS", value)
		if _, err := deliveryOptions(); err == nil {
			t.Fatal(value)
		}
	}
	t.Setenv("WATERMARK_JOB_ARCHIVE_TTL_SECONDS", "8")
	options, err := deliveryOptions()
	if err != nil || options.ArchiveTTL != 8*time.Second {
		t.Fatal(options, err)
	}
}

func TestAuditFailureIsVisibleWithoutPartialKeyMutation(t *testing.T) {
	clientRequestsMu.Lock()
	clientRequestsMap = map[string]*ClientRequests{}
	clientRequestsMu.Unlock()
	dir := filepath.Join(t.TempDir(), "data")
	master := bytes.Repeat([]byte{42}, 32)
	legacy := bytes.Repeat([]byte{7}, 32)
	s, err := distribution.Open(dir, master, legacy)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	r := gin.New()
	session := auth.New("admin", "test-password-long", false)
	authAPI := r.Group("/api/auth", auth.SameOrigin(), auditMiddleware(s))
	authAPI.POST("/login", session.Login)
	api := r.Group("/api", auth.SameOrigin(), session.Require(), auditMiddleware(s))
	registerPlatform(api, s, t.TempDir())
	registerAudit(api, s)
	login := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/auth/login", strings.NewReader(`{"username":"admin","password":"test-password-long"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(login, req)
	cookie := login.Result().Cookies()[0]
	send := func(method, path string) *httptest.ResponseRecorder {
		request := httptest.NewRequest(method, path, nil)
		request.AddCookie(cookie)
		out := httptest.NewRecorder()
		r.ServeHTTP(out, request)
		return out
	}
	canonical := filepath.Join(dir, "platform.json")
	backup := filepath.Join(dir, "platform.saved")
	before, _ := os.ReadFile(canonical)
	if err = os.Rename(canonical, backup); err != nil {
		t.Fatal(err)
	}
	if err = os.Mkdir(canonical, 0700); err != nil {
		t.Fatal(err)
	}
	if response := send("POST", "/api/platform/keys"); response.Code != 503 {
		t.Fatal(response.Code, response.Body.String())
	}
	status := send("GET", "/api/audit/status")
	if status.Code != 200 || !strings.Contains(status.Body.String(), `"degraded":true`) || status.Header().Get("X-Audit-Degraded") != "true" {
		t.Fatal(status.Code, status.Body.String())
	}
	retryLogin := httptest.NewRecorder()
	request := httptest.NewRequest("POST", "/api/auth/login", strings.NewReader(`{"username":"admin","password":"test-password-long"}`))
	request.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(retryLogin, request)
	if retryLogin.Code != 200 {
		t.Fatal("audit failure blocked authentication", retryLogin.Code)
	}
	if err = os.Remove(canonical); err != nil {
		t.Fatal(err)
	}
	if err = os.Rename(backup, canonical); err != nil {
		t.Fatal(err)
	}
	after, _ := os.ReadFile(canonical)
	if !bytes.Equal(before, after) {
		t.Fatal("failed key mutation changed canonical state")
	}
	s.Close()
	reopened, err := distribution.Open(dir, master, legacy)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	keys, _ := reopened.Keys()
	if len(keys) != 1 {
		t.Fatal(keys)
	}
	_, total, e := reopened.AuditSearch(distribution.AuditFilter{Action: "key.create", Page: 1, Size: 20}, false)
	if e != nil || total != 0 {
		t.Fatal("key mutation/audit disagreed", total, e)
	}
}
