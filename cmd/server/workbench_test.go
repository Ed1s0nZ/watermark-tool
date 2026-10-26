package main

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"github.com/gin-gonic/gin"
	"image"
	"image/png"
	"mime/multipart"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"watermark-tool/internal/auth"
	"watermark-tool/internal/blind"
	"watermark-tool/internal/distribution"
	"watermark-tool/internal/workbench"
)

func TestWorkbenchHTTP(t *testing.T) {
	for _, managed := range []bool{false, true} {
		t.Run(map[bool]string{false: "environment", true: "managed"}[managed], func(t *testing.T) {
			clientRequestsMu.Lock()
			clientRequestsMap = make(map[string]*ClientRequests)
			clientRequestsMu.Unlock()
			key := bytes.Repeat([]byte{7}, 32)
			t.Setenv("WATERMARK_BLIND_KEY", hex.EncodeToString(key))
			var store *distribution.Store
			if managed {
				var err error
				store, err = distribution.Open(filepath.Join(t.TempDir(), "data"), bytes.Repeat([]byte{42}, 32), key)
				if err != nil {
					t.Fatal(err)
				}
				defer store.Close()
			}
			dir := t.TempDir()
			r := gin.New()
			session := auth.New("admin", "test-password-long", false)
			r.POST("/login", session.Login)
			api := r.Group("/api", auth.SameOrigin(), session.Require())
			registerWorkbench(api, store, dir)
			w := httptest.NewRecorder()
			req := httptest.NewRequest("POST", "/login", strings.NewReader(`{"username":"admin","password":"test-password-long"}`))
			req.Header.Set("Content-Type", "application/json")
			r.ServeHTTP(w, req)
			cookie := w.Result().Cookies()[0]
			send := func(route string, data []byte, text, keyID string, login bool) *httptest.ResponseRecorder {
				var body bytes.Buffer
				form := multipart.NewWriter(&body)
				form.WriteField("watermark", text)
				form.WriteField("key_id", keyID)
				p, _ := form.CreateFormFile("file", "original.png")
				p.Write(data)
				form.Close()
				req := httptest.NewRequest("POST", "/api/workbench/"+route, &body)
				req.Header.Set("Content-Type", form.FormDataContentType())
				if login {
					req.AddCookie(cookie)
				}
				w := httptest.NewRecorder()
				r.ServeHTTP(w, req)
				return w
			}
			original := imageFixture()
			if got := send("verify", original, "", "", false); got.Code != 401 {
				t.Fatal(got.Code)
			}
			added := send("dual", original, "<script>测试</script>", "", true)
			if added.Code != 200 {
				t.Fatal(added.Code, added.Body.String())
			}
			out, err := png.Decode(bytes.NewReader(added.Body.Bytes()))
			if err != nil {
				t.Fatal(err)
			}
			result, err := blind.Extract(out, key)
			if err != nil || result.Watermark != "<script>测试</script>" {
				t.Fatal(result, err)
			}
			verified := send("verify", added.Body.Bytes(), "", "", true)
			var report verificationReport
			if err = json.Unmarshal(verified.Body.Bytes(), &report); err != nil || report.Status != "unregistered" || report.Source != workbench.Digest(added.Body.Bytes()) {
				t.Fatal(report, err)
			}
			negative := send("verify", original, "", "", true)
			if err = json.Unmarshal(negative.Body.Bytes(), &report); err != nil || report.Status != "not_found" {
				t.Fatal(negative.Code, report, err)
			}
			if managed {
				batch, bkey, err := store.Begin("workbench-test-0001", workbench.Digest(original), "admin", []distribution.Recipient{{ID: "1", Name: "Alice"}})
				if err != nil {
					t.Fatal(err)
				}
				img, _ := png.Decode(bytes.NewReader(original))
				marked, err := blind.Embed(img, batch.Copies[0].ID, bkey, batch.CreatedAt)
				if err != nil {
					t.Fatal(err)
				}
				store.Finish(batch.ID, "")
				var buffer bytes.Buffer
				png.Encode(&buffer, marked)
				got := send("verify", buffer.Bytes(), "", "", true)
				report = verificationReport{}
				json.Unmarshal(got.Body.Bytes(), &report)
				if got.Code != 200 || report.Status != "registered" || report.Record == nil || report.Record.Name != "Alice" {
					t.Fatal(got.Code, report)
				}
			}
			lab := send("lab", original, "实验-A", "", true)
			var experiment workbench.Report
			json.Unmarshal(lab.Body.Bytes(), &experiment)
			if lab.Code != 200 || !experiment.OriginalIntact || len(experiment.Cases) != 10 || experiment.Source != workbench.Digest(original) {
				t.Fatal(lab.Code, lab.Body.String())
			}
			if got := send("dual", original, strings.Repeat("x", 35), "", true); got.Code != 422 {
				t.Fatal(got.Code)
			}
			if got := send("verify", []byte("broken"), "", "", true); got.Code != 422 {
				t.Fatal(got.Code)
			}

			var tiny bytes.Buffer
			png.Encode(&tiny, image.NewNRGBA(image.Rect(0, 0, 8, 8)))
			gotTiny := send("verify", tiny.Bytes(), "", "", true)
			report = verificationReport{}
			json.Unmarshal(gotTiny.Body.Bytes(), &report)
			if gotTiny.Code != 200 || report.Status != "invalid" || report.Result != nil {
				t.Fatal("invalid verification", gotTiny.Code, report)
			}
			if managed {
				next, e := store.CreateKey("admin")
				if e != nil {
					t.Fatal(e)
				}
				if _, e = store.SetKeyState(next.ID, "active"); e != nil {
					t.Fatal(e)
				}
				if _, e = store.SetKeyState("legacy-env", "disabled"); e != nil {
					t.Fatal(e)
				}
				if got := send("verify", added.Body.Bytes(), "", "legacy-env", true); got.Code != 409 {
					t.Fatal("disabled", got.Code)
				}
			}
			func() {
				blindSlots <- struct{}{}
				blindSlots <- struct{}{}
				defer func() { <-blindSlots; <-blindSlots }()
				if got := send("lab", original, "busy", "", true); got.Code != 429 {
					t.Fatal("busy", got.Code)
				}
			}()
			cross := httptest.NewRequest("POST", "/api/workbench/verify", nil)
			cross.AddCookie(cookie)
			cross.Header.Set("Origin", "https://untrusted.invalid")
			denied := httptest.NewRecorder()
			r.ServeHTTP(denied, cross)
			if denied.Code != 403 {
				t.Fatal("same origin", denied.Code)
			}

			t.Setenv("WATERMARK_BLIND_KEY", "")
			if !managed {
				if got := send("verify", original, "", "", true); got.Code != 503 {
					t.Fatal(got.Code)
				}
			}
			entries, _ := os.ReadDir(dir)
			if len(entries) != 0 {
				t.Fatal("temporary files retained", entries)
			}
		})
	}
}
