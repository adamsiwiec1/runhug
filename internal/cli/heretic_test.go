package cli

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHereticSlug(t *testing.T) {
	cases := map[string]string{
		"Qwen/Qwen2.5-7B-Instruct": "qwen-qwen2.5-7b-instruct",
		"  Mistral-7B  ":           "mistral-7b",
		"/":                        "model",
		"a/b":                      "a-b",
	}
	for in, want := range cases {
		if got := hereticSlug(in); got != want {
			t.Errorf("hereticSlug(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestDashboardClientStatusAndLogs(t *testing.T) {
	status := hereticStatus{
		Model:         "Qwen/Qwen2.5-7B-Instruct",
		Status:        "training",
		TrialsDone:    2,
		TrialsTotal:   100,
		BestRefusals:  3,
		RefusalsTotal: 100,
		BestKL:        0.44,
		Done:          false,
	}
	ls := hereticLogs{Lines: []string{"a", "b"}, Offset: 2, Total: 2, Done: false}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("token") != "sekrit" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		var v any
		switch r.URL.Path {
		case "/status.json":
			v = status
		case "/logs":
			v = ls
		default:
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(v)
	}))
	defer srv.Close()

	host := &dashboardClient{base: srv.URL, token: "sekrit"}
	got, err := host.fetchStatus()
	if err != nil {
		t.Fatalf("fetchStatus: %v", err)
	}
	if got.Model != status.Model || got.BestRefusals != 3 || got.TrialsDone != 2 {
		t.Errorf("fetchStatus mismatch: %+v", got)
	}
	lg, err := host.fetchLogs(1)
	if err != nil {
		t.Fatalf("fetchLogs: %v", err)
	}
	if len(lg.Lines) != 2 || lg.Offset != 2 {
		t.Errorf("fetchLogs mismatch: %+v", lg)
	}

	badHost := &dashboardClient{base: srv.URL, token: "nope"}
	if _, err := badHost.fetchStatus(); err == nil {
		t.Errorf("expected unauthorized error with bad token")
	}
}