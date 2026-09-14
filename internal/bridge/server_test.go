package bridge

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
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
