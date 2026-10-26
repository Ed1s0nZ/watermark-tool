package main

import (
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestConcurrentRateLimit(t *testing.T) {
	gin.SetMode(gin.TestMode)
	clientRequestsMu.Lock()
	clientRequestsMap = make(map[string]*ClientRequests)
	clientRequestsMu.Unlock()
	r := gin.New()
	r.GET("/", RateLimitMiddleware(), func(c *gin.Context) { c.Status(204) })
	var group sync.WaitGroup
	statuses := make(chan int, 40)
	for i := 0; i < 40; i++ {
		group.Add(1)
		go func() {
			defer group.Done()
			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest("GET", "/", nil))
			statuses <- w.Code
		}()
	}
	group.Wait()
	close(statuses)
	accepted := 0
	for code := range statuses {
		if code == 204 {
			accepted++
		} else if code != 429 {
			t.Fatal(code)
		}
	}
	if accepted != MaxRequestsPerMinute {
		t.Fatalf("accepted %d requests", accepted)
	}
}
