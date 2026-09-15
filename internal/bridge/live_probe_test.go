package bridge

import (
	"bytes"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLiveStreamToolsAgainstRunpod(t *testing.T) {
	if os.Getenv("RUNHUG_LIVE_PROBE") != "1" {
		t.Skip("set RUNHUG_LIVE_PROBE=1")
	}
	keyPath := filepath.Join(os.Getenv("HOME"), ".config/runhug-cli/runpod.key")
	keyB, err := os.ReadFile(keyPath)
	if err != nil {
		t.Fatal(err)
	}
	key := strings.TrimSpace(string(keyB))
	ep := os.Getenv("RUNHUG_PROBE_EP")
	if ep == "" {
		ep = "vllm-yewkh67h2vyvxw"
	}
	up := "https://api.runpod.ai/v2/" + ep + "/openai/v1"
	token := "sk-runhug-probe"
	srv, base, err := Start(Config{
		UpstreamBase:  up,
		UpstreamKey:   key,
		DefaultModel:  "empero-ai/Qwythos-9B-Claude-Mythos-5-1M",
		ListenAddr:    "127.0.0.1:0",
		ExpectedToken: token,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = srv.Close() })
	t.Log("bridge", base)

	payload := `{
	  "model":"empero-ai/Qwythos-9B-Claude-Mythos-5-1M",
	  "max_tokens":64,
	  "stream":true,
	  "tools":[{"name":"get_time","description":"Get current time","input_schema":{"type":"object","properties":{"tz":{"type":"string"}}}}],
	  "messages":[{"role":"user","content":"Reply with exactly the word PONG and nothing else."}]
	}`
	req, _ := http.NewRequest(http.MethodPost, base+"/v1/messages", bytes.NewReader([]byte(payload)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", token)
	client := &http.Client{Timeout: 5 * time.Minute}
	res, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	b, _ := io.ReadAll(res.Body)
	out := string(b)
	t.Logf("status=%d", res.StatusCode)
	t.Logf("has_content_block=%v tool_use=%v text_delta=%v",
		strings.Contains(out, "content_block"),
		strings.Contains(out, "tool_use"),
		strings.Contains(out, "text_delta"))
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, "event:") {
			t.Log(line)
		}
	}
	if res.StatusCode != 200 {
		t.Fatalf("status %d body %s", res.StatusCode, truncate(out, 500))
	}
	if !strings.Contains(out, "content_block") {
		t.Fatalf("empty stream (no content_block_*): %s", truncate(out, 800))
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
