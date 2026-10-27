package main

import (
	"bytes"
	"encoding/json"
	"github.com/gin-gonic/gin"
	"image/jpeg"
	"mime/multipart"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"watermark-tool/internal/auth"
	"watermark-tool/internal/distribution"
	"watermark-tool/internal/quality"
)

func TestSingleJPEGActualResponse(t *testing.T) {
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
	registerWorkbench(api, store, t.TempDir())
	api.POST("/add-watermark", func(c *gin.Context) { selectedAlgorithm(c, t.TempDir(), true, store) })
	login := httptest.NewRecorder()
	r.ServeHTTP(login, httptest.NewRequest("POST", "/login", strings.NewReader(`{"username":"admin","password":"long-test-password"}`)))
	cookie := login.Result().Cookies()[0]
	send := func(path, options string) *httptest.ResponseRecorder {
		var body bytes.Buffer
		form := multipart.NewWriter(&body)
		p, _ := form.CreateFormFile("file", "raw.png")
		p.Write(imageFixture())
		form.WriteField("algorithm", "go-dct-v1")
		form.WriteField("watermark", "JPEG实际交付")
		form.WriteField("output", options)
		form.Close()
		req := httptest.NewRequest("POST", "/api/"+path, &body)
		req.Header.Set("Content-Type", form.FormDataContentType())
		req.AddCookie(cookie)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w
	}
	for _, path := range []string{"add-watermark", "workbench/dual"} {
		w := send(path, `{"format":"jpeg","jpeg_quality":90}`)
		if w.Code != 200 || w.Header().Get("Content-Type") != "image/jpeg" || !strings.Contains(w.Header().Get("Content-Disposition"), ".jpg") {
			t.Fatal(w.Code, w.Body.String(), w.Header())
		}
		img, e := jpeg.Decode(bytes.NewReader(w.Body.Bytes()))
		if e != nil {
			t.Fatal(e)
		}
		result, _, e := store.Verify(img, "")
		if e != nil || result.Watermark != "JPEG实际交付" {
			t.Fatal(e)
		}
		var q quality.Summary
		if json.Unmarshal([]byte(w.Header().Get("X-Watermark-Quality")), &q) != nil || q.OutputSHA256 != distribution.Digest(w.Body.Bytes()) || q.OutputBytes != int64(w.Body.Len()) || !q.Verified || q.Final == nil {
			t.Fatal(q)
		}
	}
	for _, options := range []string{`{"format":"jpeg","jpeg_quality":59}`, `{"format":"webp"}`, `{"format":"jpeg","unknown":true}`} {
		w := send("workbench/dual", options)
		if w.Code != 422 {
			t.Fatal(w.Code, w.Body.String())
		}
	}
}
