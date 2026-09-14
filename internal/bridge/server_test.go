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
