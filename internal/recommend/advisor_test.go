package recommend

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/adamsiwiec1/runhug-cli/internal/hf"
)

func TestAdviseUsesOnlyCandidates(t *testing.T) {
	var gotBody chatRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" && r.URL.Path != "/chat/completions" {
			// client appends /chat/completions to base that may already include /v1
		}
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Errorf("decode: %v", err)
		}
		_ = json.NewEncoder(w).Encode(chatResponse{
			Choices: []struct {
				Message chatMessage `json:"message"`
			}{
				{Message: chatMessage{Role: "assistant", Content: "Pick org/a — fits 16GB."}},
			},
		})
	}))
	t.Cleanup(srv.Close)

	scored := []Scored{
		{Model: hf.Model{ID: "org/a"}, Score: 1.2, Why: []string{"likes 10"}},
		{Model: hf.Model{ID: "org/b"}, Score: 1.0, Why: []string{"likes 5"}},
	}
	out, err := Advise(context.Background(), AdvisorOpts{
		BaseURL: srv.URL + "/v1",
		Model:   "test-model",
	}, "best for RAG on 16GB", scored, []string{"AMPERE_16", "ADA_24"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "org/a") {
		t.Fatalf("out %q", out)
	}
	user := gotBody.Messages[len(gotBody.Messages)-1].Content
	if !strings.Contains(user, "org/a") || !strings.Contains(user, "org/b") {
		t.Fatalf("prompt missing candidates: %q", user)
	}
	if !strings.Contains(user, "AMPERE_16") {
		t.Fatalf("prompt missing GPU hint: %q", user)
	}
	if gotBody.Model != "test-model" {
		t.Fatalf("model %q", gotBody.Model)
	}
}

func TestAdviseRequiresModel(t *testing.T) {
	_, err := Advise(context.Background(), AdvisorOpts{BaseURL: "http://127.0.0.1:9/v1"}, "q", nil, nil)
	if err == nil || !strings.Contains(err.Error(), "model required") {
		t.Fatalf("err=%v", err)
	}
}

func TestParseIntentRAG(t *testing.T) {
	in := ParseIntent("best model for RAG on a 16GB laptop", 16)
	if in.Task != "rag" {
		t.Fatalf("task %q", in.Task)
	}
	if in.MaxParamsB > 8 {
		t.Fatalf("max params too high for 16GB laptop: %v", in.MaxParamsB)
	}
}

func TestAdviseGPUOffline(t *testing.T) {
	m := hf.Model{ID: "Qwen/Qwen2.5-7B-Instruct", Tags: []string{"transformers"}}
	adv, err := AdviseGPU(m, nil, "", 8192)
	if err != nil {
		t.Fatal(err)
	}
	if adv.Choice.Pool.ID == "" || adv.Text == "" {
		t.Fatalf("%+v", adv)
	}
	if !adv.Offline {
		t.Fatal("expected offline catalog")
	}
}
