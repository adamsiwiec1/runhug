package bridge

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestBridgeMessagesRoundTrip(t *testing.T) {
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Fatalf("path %s", r.URL.Path)
		}
		if !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer up-key") {
			t.Fatalf("auth %q", r.Header.Get("Authorization"))
		}
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		if body["model"] != "served" {
			t.Fatalf("model %v", body["model"])
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id": "chatcmpl-x",
			"choices": []map[string]any{
				{"finish_reason": "stop", "message": map[string]any{"role": "assistant", "content": "pong"}},
			},
			"usage": map[string]int{"prompt_tokens": 3, "completion_tokens": 1},
		})
	}))
	t.Cleanup(up.Close)

	srv, base, err := Start(Config{
		UpstreamBase:  up.URL + "/v1",
		UpstreamKey:   "up-key",
		DefaultModel:  "served",
		ListenAddr:    "127.0.0.1:0",
		ExpectedToken: "sk-runhug-test",
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = srv.Close() })

	payload := `{"model":"claude-sonnet","max_tokens":32,"messages":[{"role":"user","content":"ping"}]}`
	req, _ := http.NewRequest(http.MethodPost, base+"/v1/messages", bytes.NewReader([]byte(payload)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", "sk-runhug-test")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	if res.StatusCode != 200 {
		t.Fatalf("%d %s", res.StatusCode, b)
	}
	var ar anthropicResp
	if err := json.Unmarshal(b, &ar); err != nil {
		t.Fatal(err)
	}
	if len(ar.Content) != 1 || ar.Content[0].Text != "pong" {
		t.Fatalf("%s", b)
	}
}

func TestBridgeAuthReject(t *testing.T) {
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not reach upstream")
	}))
	t.Cleanup(up.Close)
	srv, base, err := Start(Config{
		UpstreamBase:  up.URL + "/v1",
		ListenAddr:    "127.0.0.1:0",
		ExpectedToken: "secret",
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = srv.Close() })
	req, _ := http.NewRequest(http.MethodPost, base+"/v1/messages", strings.NewReader(`{"model":"m","max_tokens":1,"messages":[{"role":"user","content":"x"}]}`))
	req.Header.Set("x-api-key", "wrong")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != 401 {
		t.Fatalf("%d", res.StatusCode)
	}
}

func TestStripTools(t *testing.T) {
	in := []byte(`{"model":"m","messages":[{"role":"user","content":"hi"}],"tools":[{"type":"function"}],"tool_choice":"auto","parallel_tool_calls":true,"max_tokens":8}`)
	out := stripTools(in)
	var m map[string]any
	if err := json.Unmarshal(out, &m); err != nil {
		t.Fatal(err)
	}
	if _, ok := m["tools"]; ok {
		t.Fatal("tools still present")
	}
	if _, ok := m["tool_choice"]; ok {
		t.Fatal("tool_choice still present")
	}
	if _, ok := m["parallel_tool_calls"]; ok {
		t.Fatal("parallel_tool_calls still present")
	}
	if m["model"] != "m" {
		t.Fatalf("%v", m["model"])
	}
	if !openaiBodyHasTools(in) {
		t.Fatal("expected has tools")
	}
	if openaiBodyHasTools(out) {
		t.Fatal("expected no tools after strip")
	}
	if openaiBodyHasTools([]byte(`{"tools":[]}`)) {
		t.Fatal("empty tools array should be false")
	}
}

func TestBridgeRetriesWithoutToolsOnUpstream5xx(t *testing.T) {
	var mu sync.Mutex
	var saw []bool // whether each request included tools
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		has := openaiBodyHasTools(body)
		mu.Lock()
		saw = append(saw, has)
		n := len(saw)
		mu.Unlock()
		if has {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"status":500,"title":"Internal Server Error"}`))
			return
		}
		if n < 2 {
			t.Errorf("expected tools request before no-tools retry, saw=%v", saw)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id": "chatcmpl-retry",
			"choices": []map[string]any{
				{"finish_reason": "stop", "message": map[string]any{"role": "assistant", "content": "hello without tools"}},
			},
			"usage": map[string]int{"prompt_tokens": 10, "completion_tokens": 4},
		})
	}))
	t.Cleanup(up.Close)

	srv, base, err := Start(Config{
		UpstreamBase:  up.URL + "/v1",
		UpstreamKey:   "up-key",
		DefaultModel:  "served",
		ListenAddr:    "127.0.0.1:0",
		ExpectedToken: "sk-runhug-test",
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = srv.Close() })

	payload := `{
		"model":"claude-sonnet",
		"max_tokens":32,
		"tools":[{"name":"Bash","description":"run","input_schema":{"type":"object","properties":{}}}],
		"messages":[{"role":"user","content":"hi"}]
	}`
	req, _ := http.NewRequest(http.MethodPost, base+"/v1/messages", bytes.NewReader([]byte(payload)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", "sk-runhug-test")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	if res.StatusCode != 200 {
		t.Fatalf("%d %s", res.StatusCode, b)
	}
	var ar anthropicResp
	if err := json.Unmarshal(b, &ar); err != nil {
		t.Fatal(err)
	}
	if len(ar.Content) != 1 || ar.Content[0].Type != "text" || ar.Content[0].Text != "hello without tools" {
		t.Fatalf("%s", b)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(saw) != 2 || !saw[0] || saw[1] {
		t.Fatalf("expected [tools=true, tools=false], got %v", saw)
	}
}

func TestBridgeStreamRetriesWithoutToolsOnUpstream5xx(t *testing.T) {
	var mu sync.Mutex
	var saw []bool
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		has := openaiBodyHasTools(body)
		mu.Lock()
		saw = append(saw, has)
		mu.Unlock()
		if has {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte(`{"status":500,"title":"Internal Server Error"}`))
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: {\"id\":\"chatcmpl-s\",\"choices\":[{\"delta\":{\"content\":\"streamed ok\"},\"finish_reason\":null}]}\n\n"))
		_, _ = w.Write([]byte("data: {\"id\":\"chatcmpl-s\",\"choices\":[{\"delta\":{},\"finish_reason\":\"stop\"}]}\n\n"))
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}))
	t.Cleanup(up.Close)

	srv, base, err := Start(Config{
		UpstreamBase:  up.URL + "/v1",
		UpstreamKey:   "up-key",
		DefaultModel:  "served",
		ListenAddr:    "127.0.0.1:0",
		ExpectedToken: "sk-runhug-test",
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = srv.Close() })

	payload := `{
		"model":"claude-sonnet",
		"max_tokens":32,
		"stream":true,
		"tools":[{"name":"Bash","description":"run","input_schema":{"type":"object","properties":{}}}],
		"messages":[{"role":"user","content":"hi"}]
	}`
	req, _ := http.NewRequest(http.MethodPost, base+"/v1/messages", bytes.NewReader([]byte(payload)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", "sk-runhug-test")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	if res.StatusCode != 200 {
		t.Fatalf("%d %s", res.StatusCode, b)
	}
	out := string(b)
	if !strings.Contains(out, "content_block_delta") || !strings.Contains(out, "streamed ok") {
		t.Fatalf("expected content deltas, got:\n%s", out)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(saw) != 2 || !saw[0] || saw[1] {
		t.Fatalf("expected [tools=true, tools=false], got %v", saw)
	}
}

func TestBridgeStreamEmptyWithToolsRetriesThenContent(t *testing.T) {
	var mu sync.Mutex
	var n int
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		has := openaiBodyHasTools(body)
		mu.Lock()
		n++
		call := n
		mu.Unlock()
		w.Header().Set("Content-Type", "text/event-stream")
		if has {
			// HTTP 200 but empty SSE — the live failure mode before the empty-stream guard.
			_, _ = w.Write([]byte("data: {\"id\":\"chatcmpl-e\",\"choices\":[{\"delta\":{},\"finish_reason\":\"stop\"}]}\n\n"))
			_, _ = w.Write([]byte("data: [DONE]\n\n"))
			return
		}
		if call < 2 {
			t.Errorf("expected tools request before strip retry, call=%d hasTools=%v", call, has)
		}
		_, _ = w.Write([]byte("data: {\"id\":\"chatcmpl-e2\",\"choices\":[{\"delta\":{\"content\":\"recovered\"},\"finish_reason\":null}]}\n\n"))
		_, _ = w.Write([]byte("data: {\"id\":\"chatcmpl-e2\",\"choices\":[{\"delta\":{},\"finish_reason\":\"stop\"}]}\n\n"))
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}))
	t.Cleanup(up.Close)

	srv, base, err := Start(Config{
		UpstreamBase:  up.URL + "/v1",
		DefaultModel:  "served",
		ListenAddr:    "127.0.0.1:0",
		ExpectedToken: "tok",
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = srv.Close() })

	payload := `{"model":"claude","max_tokens":16,"stream":true,"tools":[{"name":"X","description":"d","input_schema":{"type":"object"}}],"messages":[{"role":"user","content":"hi"}]}`
	req, _ := http.NewRequest(http.MethodPost, base+"/v1/messages", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", "tok")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	if res.StatusCode != 200 {
		t.Fatalf("%d %s", res.StatusCode, b)
	}
	if !strings.Contains(string(b), "recovered") || !strings.Contains(string(b), "content_block_delta") {
		t.Fatalf("got %s", b)
	}
}

func TestBridgeStreamEmptyGuardNoSilentSuccess(t *testing.T) {
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte("data: {\"id\":\"chatcmpl-z\",\"choices\":[{\"delta\":{},\"finish_reason\":\"stop\"}]}\n\n"))
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
	}))
	t.Cleanup(up.Close)

	srv, base, err := Start(Config{
		UpstreamBase:  up.URL + "/v1",
		DefaultModel:  "served",
		ListenAddr:    "127.0.0.1:0",
		ExpectedToken: "tok",
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = srv.Close() })

	// Tools present → buffer path → empty after strip retry → Anthropic error, not empty SSE.
	payload := `{"model":"claude","max_tokens":8,"stream":true,"tools":[{"name":"X","description":"d","input_schema":{"type":"object"}}],"messages":[{"role":"user","content":"hi"}]}`
	req, _ := http.NewRequest(http.MethodPost, base+"/v1/messages", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", "tok")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	if res.StatusCode == 200 && strings.Contains(string(b), "message_start") && !strings.Contains(string(b), "content_block") {
		t.Fatalf("must not emit empty successful Anthropic SSE: %s", b)
	}
	if res.StatusCode == 200 {
		t.Fatalf("expected error status for empty stream, got 200: %s", b)
	}
	if !strings.Contains(string(b), "empty stream") {
		t.Fatalf("expected empty-stream error, got %d %s", res.StatusCode, b)
	}
}

func TestOpenAIStreamEmpty(t *testing.T) {
	if !openAIStreamEmpty(nil) {
		t.Fatal("nil")
	}
	emptySSE := []byte("data: {\"choices\":[{\"delta\":{},\"finish_reason\":\"stop\"}]}\n\ndata: [DONE]\n\n")
	if !openAIStreamEmpty(emptySSE) {
		t.Fatal("empty sse")
	}
	withText := []byte("data: {\"choices\":[{\"delta\":{\"content\":\"hi\"}}]}\n\ndata: [DONE]\n\n")
	if openAIStreamEmpty(withText) {
		t.Fatal("text sse")
	}
	withTools := []byte("data: {\"choices\":[{\"delta\":{\"tool_calls\":[{\"index\":0,\"id\":\"1\",\"function\":{\"name\":\"x\",\"arguments\":\"{}\"}}]}}]}\n\n")
	if openAIStreamEmpty(withTools) {
		t.Fatal("tools sse")
	}
}

func TestPipeOpenAISSEMapsReasoningToText(t *testing.T) {
	in := strings.Join([]string{
		`data: {"id":"chatcmpl-r","choices":[{"delta":{"role":"assistant","content":""}}]}`,
		``,
		`data: {"id":"chatcmpl-r","choices":[{"delta":{"reasoning":"Hello "}}]}`,
		``,
		`data: {"id":"chatcmpl-r","choices":[{"delta":{"reasoning":"world"},"finish_reason":"stop"}]}`,
		``,
		`data: [DONE]`,
		``,
	}, "\n")
	var buf bytes.Buffer
	if err := PipeOpenAISSE(strings.NewReader(in), &buf, "m"); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "content_block_start") || !strings.Contains(out, "text_delta") {
		t.Fatalf("expected text blocks from reasoning: %s", out)
	}
	if !strings.Contains(out, "Hello ") || !strings.Contains(out, "world") {
		t.Fatalf("missing reasoning text: %s", out)
	}
}

func TestOpenAIStreamEmptyReasoningNotEmpty(t *testing.T) {
	sse := []byte("data: {\"choices\":[{\"delta\":{\"reasoning\":\"think\"}}]}\n\ndata: [DONE]\n\n")
	if openAIStreamEmpty(sse) {
		t.Fatal("reasoning-only stream must not be treated as empty")
	}
}

func TestBridgeStreamToolsFirstByteBeforeUpstreamEOF(t *testing.T) {
	started := make(chan struct{})
	up := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fl, _ := w.(http.Flusher)
		_, _ = w.Write([]byte("data: {\"id\":\"chatcmpl-slow\",\"choices\":[{\"delta\":{\"reasoning\":\"Hello\"}}]}\n\n"))
		if fl != nil {
			fl.Flush()
		}
		close(started)
		// Hold the rest of the stream so a full-buffer implementation cannot finish.
		select {
		case <-r.Context().Done():
		case <-time.After(3 * time.Second):
			_, _ = w.Write([]byte("data: {\"id\":\"chatcmpl-slow\",\"choices\":[{\"delta\":{},\"finish_reason\":\"stop\"}]}\n\n"))
			_, _ = w.Write([]byte("data: [DONE]\n\n"))
		}
	}))
	t.Cleanup(up.Close)

	srv, base, err := Start(Config{
		UpstreamBase:  up.URL + "/v1",
		DefaultModel:  "served",
		ListenAddr:    "127.0.0.1:0",
		ExpectedToken: "tok",
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = srv.Close() })

	payload := `{"model":"claude","max_tokens":32,"stream":true,"tools":[{"name":"X","description":"d","input_schema":{"type":"object"}}],"messages":[{"role":"user","content":"hi"}]}`
	req, _ := http.NewRequest(http.MethodPost, base+"/v1/messages", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", "tok")
	client := &http.Client{Timeout: 2 * time.Second}
	t0 := time.Now()
	res, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		b, _ := io.ReadAll(res.Body)
		t.Fatalf("%d %s", res.StatusCode, b)
	}
	buf := make([]byte, 4096)
	n, err := res.Body.Read(buf)
	elapsed := time.Since(t0)
	if n == 0 {
		t.Fatalf("no first byte: err=%v elapsed=%s", err, elapsed)
	}
	got := string(buf[:n])
	if !strings.Contains(got, "message_start") && !strings.Contains(got, "text_delta") && !strings.Contains(got, "Hello") {
		t.Fatalf("unexpected first chunk after %s: %q", elapsed, got)
	}
	if elapsed > 1500*time.Millisecond {
		t.Fatalf("first Anthropic SSE too slow (%s); bridge likely buffered the full upstream stream", elapsed)
	}
	select {
	case <-started:
	default:
		t.Fatal("upstream did not start")
	}
}
