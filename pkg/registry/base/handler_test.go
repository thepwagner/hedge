package base_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/thepwagner/hedge/pkg/cached"
	"github.com/thepwagner/hedge/pkg/observability"
	"github.com/thepwagner/hedge/pkg/registry/base"
	"github.com/thepwagner/hedge/proto/hedge/v1"
)

func TestNewHandler(t *testing.T) {
	storage := cached.InMemory[string, []byte]()
	h := base.NewHandler(observability.NoopTracer, storage)

	var ctr uint64
	h.Register("/key/{key}", 1*time.Minute, func(ctx context.Context, req base.HttpRequest) (*hedge.HttpResponse, error) {
		body, _ := json.Marshal(map[string]interface{}{
			"key":     req.PathVars["key"],
			"counter": atomic.AddUint64(&ctr, 1),
		})
		return &hedge.HttpResponse{
			StatusCode:  http.StatusTeapot,
			ContentType: "application/json",
			Body:        body,
		}, nil
	})

	res := httptest.NewRecorder()
	h.ServeHTTP(res, httptest.NewRequest("GET", "/key/foo", nil))
	assert.Equal(t, http.StatusTeapot, res.Code)
	assert.Equal(t, "application/json", res.Header().Get("Content-Type"))
	assert.Equal(t, `{"counter":1,"key":"foo"}`, res.Body.String())

	res = httptest.NewRecorder()
	h.ServeHTTP(res, httptest.NewRequest("GET", "/key/foo", nil))
	assert.Equal(t, http.StatusTeapot, res.Code)
	assert.Equal(t, "application/json", res.Header().Get("Content-Type"))
	assert.Equal(t, `{"counter":1,"key":"foo"}`, res.Body.String())

	res = httptest.NewRecorder()
	h.ServeHTTP(res, httptest.NewRequest("GET", "/key/bar", nil))
	assert.Equal(t, http.StatusTeapot, res.Code)
	assert.Equal(t, "application/json", res.Header().Get("Content-Type"))
	assert.Equal(t, `{"counter":2,"key":"bar"}`, res.Body.String())
}
