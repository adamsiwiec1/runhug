package cli

import (
	"bytes"
	"errors"
	"flag"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/adamsiwiec/runpod-vllm-proxy/internal/config"
)

func TestConnectHelp(t *testing.T) {
	setupConnectTest(t)
	err := cmdConnect([]string{"-h"})
	if !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("help: %v", err)
	}
}

func TestConnectRejectsBrowserFlags(t *testing.T) {
	setupConnectTest(t)
	for _, name := range []string{"--browser", "--no-browser"} {
		err := cmdConnect([]string{name, "--key", "k"})
		if err == nil {
			t.Fatalf("%s: expected unknown flag", name)
		}
	}
}

func TestConnectKeySaves(t *testing.T) {
	setupConnectTest(t)
	const secret = "rvp-test-key-never-print"
	out := captureStdout(t, func() {
		if err := cmdConnect([]string{"--key", secret}); err != nil {
			t.Fatal(err)
		}
	})
	if strings.Contains(out, secret) {
		t.Fatal("must not print the API key")
	}
	if !strings.Contains(out, "Connected") || !strings.Contains(out, "0600") {
		t.Fatalf("output %q", out)
	}
	if !strings.Contains(out, "never printed") {
		t.Fatalf("output %q", out)
	}
	if config.Load().RunpodAPIKey != secret {
		t.Fatal("stored")
	}
	path, err := config.StoredKeyPath()
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
}

func TestConnectRejectedKeyNotSaved(t *testing.T) {
	setupConnectTest(t)
	verifyKey = func(string) error {
		return errors.New("runpod: HTTP 401: unauthorized")
	}
	if err := config.SaveKey("old-key"); err != nil {
		t.Fatal(err)
	}
	err := cmdConnect([]string{"--key", "bad-key"})
	if err == nil || !strings.Contains(err.Error(), "rejected") {
		t.Fatalf("err %v", err)
	}
	if config.Load().RunpodAPIKey != "old-key" {
		t.Fatal("must keep previous key")
	}
}

func TestConnectVerifyFailureNotSaved(t *testing.T) {
	setupConnectTest(t)
	verifyKey = func(string) error {
		return errors.New("runpod: HTTP 503: unavailable")
	}
	err := cmdConnect([]string{"--key", "maybe-good"})
	if err == nil || !strings.Contains(err.Error(), "could not verify") {
		t.Fatalf("err %v", err)
	}
	if config.HasStoredKey() {
		t.Fatal("must not save")
	}
}

func TestConnectHeadlessPrintsURL(t *testing.T) {
	setupConnectTest(t)
	var err error
	out := captureStdout(t, func() {
		err = cmdConnect(nil)
	})
	if err == nil || !strings.Contains(err.Error(), config.EnvRunpodAPIKey) {
		t.Fatalf("err %v", err)
	}
	if !strings.Contains(out, runpodAPIKeysURL) {
		t.Fatalf("missing console URL\n%s", out)
	}
	if !strings.Contains(out, "https://console.runpod.io/user/credentials?tab=api-key") {
		t.Fatalf("missing exact credentials URL\n%s", out)
	}
	if strings.Contains(out, "/user/settings") || strings.Contains(out, "Opened") || strings.Contains(out, "browser") {
		t.Fatalf("stale browser UX\n%s", out)
	}
	if !strings.Contains(out, "no OAuth") {
		t.Fatalf("output %q", out)
	}
}

func TestConnectAlreadyConnected(t *testing.T) {
	setupConnectTest(t)
	if err := config.SaveKey("stored-secret"); err != nil {
		t.Fatal(err)
	}
	out := captureStdout(t, func() {
		if err := cmdConnect(nil); err != nil {
			t.Fatal(err)
		}
	})
	if strings.Contains(out, "stored-secret") {
		t.Fatal("must not print the stored key")
	}
	if !strings.Contains(out, "Connected") || !strings.Contains(out, "stored key") {
		t.Fatalf("output %q", out)
	}
	if !strings.Contains(out, runpodAPIKeysURL) {
		t.Fatalf("missing URL\n%s", out)
	}
	if !strings.Contains(out, "pass --key") {
		t.Fatalf("output %q", out)
	}
}

func TestConnectAlreadyConnectedEnv(t *testing.T) {
	setupConnectTest(t)
	t.Setenv(config.EnvRunpodAPIKey, "env-secret")
	out := captureStdout(t, func() {
		if err := cmdConnect(nil); err != nil {
			t.Fatal(err)
		}
	})
	if strings.Contains(out, "env-secret") {
		t.Fatal("must not print the env key")
	}
	if !strings.Contains(out, config.EnvRunpodAPIKey) {
		t.Fatalf("output %q", out)
	}
	if config.HasStoredKey() {
		t.Fatal("must not copy env to disk without asking")
	}
}

func TestConnectReplaceStoredKey(t *testing.T) {
	setupConnectTest(t)
	if err := config.SaveKey("old-key"); err != nil {
		t.Fatal(err)
	}
	promptOK = func() bool { return true }
	askReplace = func() (bool, error) { return true, nil }
	readAPIKey = func() (string, error) { return "new-key", nil }
	out := captureStdout(t, func() {
		if err := cmdConnect(nil); err != nil {
			t.Fatal(err)
		}
	})
	if config.Load().RunpodAPIKey != "new-key" {
		t.Fatal("replaced")
	}
	if strings.Contains(out, "old-key") || strings.Contains(out, "new-key") {
		t.Fatal("must not print the API key")
	}
	if !strings.Contains(out, runpodAPIKeysURL) {
		t.Fatalf("missing URL\n%s", out)
	}
}

func TestConnectKeepStoredKey(t *testing.T) {
	setupConnectTest(t)
	if err := config.SaveKey("keep-me"); err != nil {
		t.Fatal(err)
	}
	promptOK = func() bool { return true }
	askReplace = func() (bool, error) { return false, nil }
	out := captureStdout(t, func() {
		if err := cmdConnect(nil); err != nil {
			t.Fatal(err)
		}
	})
	if !strings.Contains(out, "Kept the existing key") {
		t.Fatalf("output %q", out)
	}
	if strings.Contains(out, "keep-me") {
		t.Fatal("must not print the key")
	}
	if config.Load().RunpodAPIKey != "keep-me" {
		t.Fatal("kept")
	}
}

func TestConnectEnvStillWins(t *testing.T) {
	setupConnectTest(t)
	t.Setenv(config.EnvRunpodAPIKey, "env-secret")
	out := captureStdout(t, func() {
		if err := cmdConnect([]string{"--key", "disk-secret"}); err != nil {
			t.Fatal(err)
		}
	})
	if strings.Contains(out, "env-secret") || strings.Contains(out, "disk-secret") {
		t.Fatal("must not print keys")
	}
	if !strings.Contains(out, "still wins") {
		t.Fatalf("output %q", out)
	}
	raw, err := os.ReadFile(mustStoredPath(t))
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(raw)) != "disk-secret" {
		t.Fatalf("file %q", raw)
	}
	if config.Load().RunpodAPIKey != "env-secret" {
		t.Fatal("env first")
	}
}

func TestConnectSanitizesCRLFKey(t *testing.T) {
	setupConnectTest(t)
	const dirty = "rpa_testkey123\n\r"
	var verified string
	verifyKey = func(k string) error {
		if verified != "" {
			t.Fatal("verify must run once")
		}
		verified = k
		return nil
	}
	out := captureStdout(t, func() {
		if err := cmdConnect([]string{"--key", dirty}); err != nil {
			t.Fatal(err)
		}
	})
	if verified != "rpa_testkey123" {
		t.Fatalf("verify got %q", verified)
	}
	if strings.Contains(out, "rpa_testkey123") || strings.Contains(out, dirty) {
		t.Fatal("must not print the API key")
	}
	if config.Load().RunpodAPIKey != "rpa_testkey123" {
		t.Fatalf("stored %q", config.Load().RunpodAPIKey)
	}
}

func TestConnectRejectsControlOnlyKey(t *testing.T) {
	setupConnectTest(t)
	verifyKey = func(string) error {
		t.Fatal("must not verify empty key")
		return nil
	}
	err := cmdConnect([]string{"--key", "\n\r\x00"})
	if err == nil || !strings.Contains(err.Error(), "empty API key") {
		t.Fatalf("err %v", err)
	}
	if strings.Contains(err.Error(), "rpa_") {
		t.Fatal("must not print a key")
	}
	if config.HasStoredKey() {
		t.Fatal("must not save")
	}
}

func TestIsUnauthorized(t *testing.T) {
	if !isUnauthorized(errors.New("runpod: HTTP 401: Unauthorized")) {
		t.Fatal("401")
	}
	if isUnauthorized(errors.New("runpod: HTTP 503: unavailable")) {
		t.Fatal("503")
	}
}

func setupConnectTest(t *testing.T) {
	t.Helper()
	origVerify, origPrompt, origAsk, origRead := verifyKey, promptOK, askReplace, readAPIKey
	t.Cleanup(func() {
		verifyKey, promptOK, askReplace, readAPIKey = origVerify, origPrompt, origAsk, origRead
	})
	t.Setenv("RVP_CONFIG", filepath.Join(t.TempDir(), "registry.json"))
	t.Setenv(config.EnvRunpodAPIKey, "")
	t.Setenv("NO_COLOR", "1")
	verifyKey = func(string) error { return nil }
	promptOK = func() bool { return false }
	askReplace = func() (bool, error) { t.Fatal("askReplace"); return false, nil }
	readAPIKey = func() (string, error) { t.Fatal("readAPIKey"); return "", nil }
}

func mustStoredPath(t *testing.T) string {
	t.Helper()
	path, err := config.StoredKeyPath()
	if err != nil {
		t.Fatal(err)
	}
	return path
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	old := os.Stdout
	os.Stdout = w
	defer func() { os.Stdout = old }()
	fn()
	_ = w.Close()
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatal(err)
	}
	_ = r.Close()
	return buf.String()
}
