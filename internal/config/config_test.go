package config

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadPrefersEnvOverStored(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("RVP_CONFIG", filepath.Join(dir, "registry.json"))
	t.Setenv(EnvHFToken, "")
	if err := SaveKey("stored-key"); err != nil {
		t.Fatal(err)
	}
	t.Setenv(EnvRunpodAPIKey, "env-key")
	env := Load()
	if env.RunpodAPIKey != "env-key" {
		t.Fatalf("env first: %q", env.RunpodAPIKey)
	}
	t.Setenv(EnvRunpodAPIKey, "")
	env = Load()
	if env.RunpodAPIKey != "stored-key" {
		t.Fatalf("stored: %q", env.RunpodAPIKey)
	}
}

func TestSaveKeyPermissionsAndDelete(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("RVP_CONFIG", filepath.Join(dir, "registry.json"))
	t.Setenv(EnvRunpodAPIKey, "")
	if err := SaveKey("  secret-value  "); err != nil {
		t.Fatal(err)
	}
	path, err := StoredKeyPath()
	if err != nil {
		t.Fatal(err)
	}
	st, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if st.Mode().Perm() != 0o600 {
		t.Fatalf("perm %o", st.Mode().Perm())
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != "secret-value\n" {
		t.Fatalf("file %q", raw)
	}
	if Load().RunpodAPIKey != "secret-value" {
		t.Fatal("load after save")
	}
	if err := DeleteKey(); err != nil {
		t.Fatal(err)
	}
	if Load().RunpodAPIKey != "" {
		t.Fatal("expected empty after delete")
	}
	if err := DeleteKey(); err != nil {
		t.Fatal(err)
	}
}

func TestRequireRunpodMentionsConnect(t *testing.T) {
	t.Setenv("RVP_CONFIG", filepath.Join(t.TempDir(), "registry.json"))
	t.Setenv(EnvRunpodAPIKey, "")
	err := Load().RequireRunpod()
	if err == nil {
		t.Fatal("expected error")
	}
	msg := err.Error()
	if !strings.Contains(msg, "connect") || !strings.Contains(msg, EnvRunpodAPIKey) {
		t.Fatalf("%v", err)
	}
}

func TestSaveKeyRejectsEmpty(t *testing.T) {
	t.Setenv("RVP_CONFIG", filepath.Join(t.TempDir(), "registry.json"))
	if err := SaveKey("  "); err == nil {
		t.Fatal("expected error")
	}
	if err := SaveKey("\n\r\x1b"); err == nil {
		t.Fatal("expected error for controls")
	}
}

func TestSanitizeAPIKeyStripsCRLFToValidHeaderToken(t *testing.T) {
	const fake = "rpa_testkey123"
	got := SanitizeAPIKey("  Bearer " + fake + "\n\r")
	if got != fake {
		t.Fatalf("got %q", got)
	}
	if SanitizeAPIKey(fake+"\n") != fake || SanitizeAPIKey(fake+"\r") != fake {
		t.Fatal("trailing CR/LF")
	}
	if SanitizeAPIKey("\x00"+fake+"\x1b") != fake {
		t.Fatal("ascii controls")
	}
	if SanitizeAPIKey("\n\r") != "" {
		t.Fatal("empty after sanitize")
	}

	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(srv.Close)

	req, err := http.NewRequest(http.MethodGet, srv.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", "Bearer "+got)
	res, err := srv.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if gotAuth != "Bearer "+fake {
		t.Fatalf("auth %q", gotAuth)
	}
}

func TestLoadSanitizesDirtyStoredKey(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("RVP_CONFIG", filepath.Join(dir, "registry.json"))
	t.Setenv(EnvRunpodAPIKey, "")
	path, err := StoredKeyPath()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("rpa_testkey123\r\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if Load().RunpodAPIKey != "rpa_testkey123" {
		t.Fatalf("load %q", Load().RunpodAPIKey)
	}
}

func TestLoadSanitizesEnvKey(t *testing.T) {
	t.Setenv("RVP_CONFIG", filepath.Join(t.TempDir(), "registry.json"))
	t.Setenv(EnvHFToken, "")
	t.Setenv(EnvRunpodAPIKey, "rpa_testkey123\n")
	if Load().RunpodAPIKey != "rpa_testkey123" {
		t.Fatalf("env %q", Load().RunpodAPIKey)
	}
}
