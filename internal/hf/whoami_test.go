package hf

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWhoami(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/whoami-v2" {
			t.Fatalf("path %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-token" {
			t.Fatalf("auth %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"name":"alice","type":"user"}`))
	}))
	t.Cleanup(srv.Close)

	c := New("test-token")
	c.BaseURL = srv.URL
	c.HTTP = srv.Client()
	name, err := c.Whoami(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if name != "alice" {
		t.Fatalf("got %q", name)
	}
}

func TestWhoamiUnauthorized(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"Invalid username or password."}`))
	}))
	t.Cleanup(srv.Close)

	c := New("bad")
	c.BaseURL = srv.URL
	c.HTTP = srv.Client()
	_, err := c.Whoami(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
}
