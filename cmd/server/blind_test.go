package main

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"mime/multipart"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"watermark-tool/internal/auth"
	"watermark-tool/internal/blind"
)

func imageFixture() []byte {
	img := image.NewNRGBA(image.Rect(0, 0, 512, 512))
	for y := 0; y < 512; y++ {
		for x := 0; x < 512; x++ {
			img.SetNRGBA(x, y, color.NRGBA{uint8(x%190 + 30), uint8(y%190 + 30), 100, 255})
		}
	}
	var buffer bytes.Buffer
	png.Encode(&buffer, img)
	return buffer.Bytes()
}
func TestBlindHTTP(t *testing.T) {
	gin.SetMode(gin.TestMode)
	key := bytes.Repeat([]byte{9}, 32)
	t.Setenv("WATERMARK_BLIND_KEY", hex.EncodeToString(key))
	r := gin.New()
	session := auth.New("admin", "test-password-long", false)
	dir := t.TempDir()
	r.POST("/login", session.Login)
	api := r.Group("/api", auth.SameOrigin(), session.Require())
	api.POST("/add", func(c *gin.Context) {
		if !selectedAlgorithm(c, dir, true) {
			c.Status(204)
		}
	})
	api.POST("/extract", func(c *gin.Context) {
		if !selectedAlgorithm(c, dir, false) {
			c.Status(204)
		}
	})
	login := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/login", strings.NewReader(`{"username":"admin","password":"test-password-long"}`))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(login, req)
	cookie := login.Result().Cookies()[0]
	send := func(path, mode, text string, data []byte, authenticated bool) *httptest.ResponseRecorder {
		var body bytes.Buffer
		form := multipart.NewWriter(&body)
		form.WriteField("algorithm", mode)
		form.WriteField("watermark", text)
		part, _ := form.CreateFormFile("file", "input.png")
		part.Write(data)
		form.Close()
		req := httptest.NewRequest("POST", path, &body)
		req.Header.Set("Content-Type", form.FormDataContentType())
		if authenticated {
			req.AddCookie(cookie)
		}
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w
	}
	original := imageFixture()
	if w := send("/api/add", blind.Algorithm, "test", original, false); w.Code != 401 {
		t.Fatal(w.Code)
	}
	if w := send("/api/add", "legacy", "test", original, true); w.Code != 204 {
		t.Fatal(w.Code)
	}
	if w := send("/api/add", "bad", "test", original, true); w.Code != 400 {
		t.Fatal(w.Code)
	}
	added := send("/api/add", blind.Algorithm, "测试-A100", original, true)
	if added.Code != 200 {
		t.Fatal(added.Code, added.Body.String())
	}
	if added.Header().Get("X-Watermark-Algorithm") != blind.Algorithm {
		t.Fatal("missing algorithm header")
	}
	extracted := send("/api/extract?show_timestamp=true", blind.Algorithm, "", added.Body.Bytes(), true)
	if extracted.Code != 200 {
		t.Fatal(extracted.Code, extracted.Body.String())
	}
	var result blind.Result
	if err := json.NewDecoder(extracted.Body).Decode(&result); err != nil || result.Watermark != "测试-A100" || result.Timestamp == "" || result.Algorithm != blind.Algorithm {
		t.Fatal(result, err)
	}
	if w := send("/api/extract", blind.Algorithm, "", original, true); w.Code != 422 {
		t.Fatal(w.Code)
	}
	if w := send("/api/add", blind.Algorithm, strings.Repeat("x", 35), original, true); w.Code != 422 {
		t.Fatal(w.Code)
	}
	t.Setenv("WATERMARK_BLIND_KEY", "")
	if w := send("/api/add", blind.Algorithm, "test", original, true); w.Code != 503 {
		t.Fatal(w.Code)
	}
	if w := send("/api/add", "legacy", "test", original, true); w.Code != 204 {
		t.Fatal("missing blind key broke old mode")
	}
	t.Setenv("WATERMARK_BLIND_KEY", hex.EncodeToString(key))
	blindSlots <- struct{}{}
	blindSlots <- struct{}{}
	w := send("/api/add", blind.Algorithm, "test", original, true)
	<-blindSlots
	<-blindSlots
	if w.Code != 429 {
		t.Fatal(w.Code)
	}
	files, err := os.ReadDir(dir)
	if err != nil || len(files) != 0 {
		t.Fatal("request temporary files not cleaned", err)
	}
}
