package runpod

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestClientAuthorizationSanitizesCRLF(t *testing.T) {
	const fake = "rpa_testkey123"
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"endpoints":[]}`)
	}))
	t.Cleanup(srv.Close)

	c := New(fake + "\n\r")
	c.BaseURL = srv.URL
	if c.APIKey != fake {
		t.Fatalf("stored key %q", c.APIKey)
	}
	_, err := c.ListEndpoints(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	if gotAuth != "Bearer "+fake {
		t.Fatalf("Authorization %q", gotAuth)
	}
}

func TestDirtyKeyIsInvalidHeaderBeforeSanitize(t *testing.T) {
	dirty := "Bearer rpa_testkey123\n"
	req, err := http.NewRequest(http.MethodGet, "http://127.0.0.1/", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", dirty)
	_, err = http.DefaultTransport.RoundTrip(req)
	if err == nil || !strings.Contains(err.Error(), "invalid header") {
		t.Fatalf("expected invalid header, got %v", err)
	}
	if strings.Contains(err.Error(), "rpa_testkey123") {
		t.Fatal("must not print the key")
	}
}
