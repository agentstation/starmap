# Provider dispatch failure probe

This Go source is the exact test overlay from the recorded failure.

```go
package app

import (
	"encoding/json/v2"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/agentstation/starport/internal/storage"
	"github.com/agentstation/starport/internal/usage"
	"github.com/stretchr/testify/require"
)

// The overlay changes only the performance fixture's key limit, upstream
// handler, timing-channel capacity, and access to its real storage adapter.
var csp122Provider http.HandlerFunc
var csp122Store storage.KVStore

func TestCSP122ProviderDispatchRespectsExclusiveTokenCapacity(t *testing.T) {
	arrivals := make(chan struct{}, 2)
	release := make(chan struct{})
	finish := sync.OnceFunc(func() { close(release) })
	var calls atomic.Int64
	csp122Provider = func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "read request", http.StatusBadRequest)
			return
		}
		var request struct {
			Model     string `json:"model"`
			MaxTokens int    `json:"max_tokens"`
		}
		if json.Unmarshal(body, &request) != nil || request.Model != "gpt-4o-mini" || request.MaxTokens != 600 {
			http.Error(w, "unexpected dispatch", http.StatusBadRequest)
			return
		}
		calls.Add(1)
		arrivals <- struct{}{}
		select {
		case <-release:
		case <-r.Context().Done():
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"id":"budget-probe","object":"chat.completion","model":"gpt-4o-mini","choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"length"}],"usage":{"prompt_tokens":1,"completion_tokens":600,"total_tokens":601}}`)
	}
	f := newPerformanceFixture(t, 0)
	var workers sync.WaitGroup
	t.Cleanup(func() { finish(); workers.Wait() })
	type result struct {
		status int
		body   string
		err    error
	}
	results := make(chan result, 2)
	dispatch := func() {
		workers.Go(func() {
			request, err := http.NewRequestWithContext(t.Context(), http.MethodPost,
				f.gateway.URL+"/v1/chat/completions",
				strings.NewReader(`{"model":"openai/gpt-4o-mini","max_tokens":600,"messages":[{"role":"user","content":"Hello"}]}`))
			if err != nil {
				results <- result{err: err}
				return
			}
			request.Header.Set("Authorization", "Bearer "+performanceGatewayKey)
			request.Header.Set("Content-Type", "application/json")
			response, err := f.client.Do(request)
			if err != nil {
				results <- result{err: err}
				return
			}
			defer response.Body.Close()
			body, err := io.ReadAll(response.Body)
			results <- result{status: response.StatusCode, body: string(body), err: err}
		})
	}
	dispatch()
	select {
	case <-arrivals:
	case first := <-results:
		t.Fatalf("first request did not reach provider: %+v", first)
	case <-time.After(10 * time.Second):
		t.Fatal("first provider dispatch did not arrive")
	}
	dispatch()
	var completed []result
	select {
	case <-arrivals:
	case refused := <-results:
		completed = append(completed, refused)
	case <-time.After(10 * time.Second):
		t.Fatal("second request neither dispatched nor returned a budget decision")
	}
	finish()
	for len(completed) < 2 {
		select {
		case response := <-results:
			completed = append(completed, response)
		case <-time.After(10 * time.Second):
			t.Fatal("request did not finish")
		}
	}
	for _, response := range completed {
		require.NoError(t, response.err)
	}
	repository, err := usage.Open(csp122Store, usage.Options{})
	require.NoError(t, err)
	var total usage.Totals
	require.Eventually(t, func() bool {
		var readErr error
		total, readErr = repository.Totals(t.Context(), usage.KeyScope("STARPORT_TEST"), usage.IntervalDay, time.Now())
		return readErr == nil && total.Tokens == calls.Load()*601
	}, 5*time.Second, 10*time.Millisecond)
	t.Logf("provider_dispatches=%d durable_tokens=%d configured_limit=1000 statuses=%v", calls.Load(), total.Tokens, []int{completed[0].status, completed[1].status})
	require.LessOrEqual(t, calls.Load(), int64(1), "concurrent dispatches must reserve distinct token capacity")
	require.LessOrEqual(t, total.Tokens, int64(1000), fmt.Sprintf("durable accounting exceeds the configured budget: %+v", completed))
}
```
