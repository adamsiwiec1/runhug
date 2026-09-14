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

func TestSanitizeAPIKeyStripsBOMAndNonASCII(t *testing.T) {
	fake := "rpa_testkey_bom"
	got := SanitizeAPIKey("\ufeff" + fake)
	if got != fake {
		t.Fatalf("BOM: got %q", got)
	}
	got = SanitizeAPIKey(fake + "\u200b")
	if got != fake {
		t.Fatalf("zwsp: got %q", got)
	}
	got = SanitizeAPIKey("Bearer\t" + fake + "\n")
	if got != fake {
		t.Fatalf("bearer+tab+nl: got %q", got)
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

func TestConfigPathOverridePrefersRUNHUG(t *testing.T) {
	t.Setenv(EnvConfig, "/tmp/runhug/registry.json")
	t.Setenv(EnvConfigLegacy, "/tmp/legacy/registry.json")
	if got := ConfigPathOverride(); got != "/tmp/runhug/registry.json" {
		t.Fatalf("got %q", got)
	}
	t.Setenv(EnvConfig, "")
	if got := ConfigPathOverride(); got != "/tmp/legacy/registry.json" {
		t.Fatalf("legacy got %q", got)
	}
}

func TestDirUsesXDGConfigHome(t *testing.T) {
	base := t.TempDir()
	t.Setenv(EnvConfig, "")
	t.Setenv(EnvConfigLegacy, "")
	t.Setenv("XDG_CONFIG_HOME", base)
	dir, err := Dir()
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(base, "runhug-cli")
	if dir != want {
		t.Fatalf("got %q want %q", dir, want)
	}
}

func TestDirFallsBackToHomeDotConfig(t *testing.T) {
	home := t.TempDir()
	t.Setenv(EnvConfig, "")
	t.Setenv(EnvConfigLegacy, "")
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("HOME", home)
	dir, err := Dir()
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(home, ".config", "runhug-cli")
	if dir != want {
		t.Fatalf("got %q want %q", dir, want)
	}
}

func TestMigrateFileIfMissingFromLegacy(t *testing.T) {
	home := t.TempDir()
	xdg := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", xdg)
	t.Setenv(EnvConfig, "")
	t.Setenv(EnvConfigLegacy, "")
	t.Setenv(EnvRunpodAPIKey, "")

	legacy := filepath.Join(home, "Library", "Application Support", "runpod-vllm-proxy")
	if err := os.MkdirAll(legacy, 0o700); err != nil {
		t.Fatal(err)
	}
	legacyKey := filepath.Join(legacy, "runpod.key")
	if err := os.WriteFile(legacyKey, []byte("migrated-secret\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	dir, err := Dir()
	if err != nil {
		t.Fatal(err)
	}
	if err := MigrateFileIfMissing(dir, "runpod.key"); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(dir, "runpod.key")
	raw, err := os.ReadFile(dest)
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != "migrated-secret\n" {
		t.Fatalf("dest %q", raw)
	}
	if Load().RunpodAPIKey != "migrated-secret" {
		t.Fatalf("load after migrate %q", Load().RunpodAPIKey)
	}
}

func TestLoadUsesRUNHUGConfigOverride(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(EnvConfig, filepath.Join(dir, "registry.json"))
	t.Setenv(EnvConfigLegacy, "")
	t.Setenv(EnvRunpodAPIKey, "")
	if err := SaveKey("from-runhug"); err != nil {
		t.Fatal(err)
	}
	if Load().RunpodAPIKey != "from-runhug" {
		t.Fatalf("got %q", Load().RunpodAPIKey)
	}
}

func TestLoadHFTokenPrefersEnvOverStored(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(EnvConfig, filepath.Join(dir, "registry.json"))
	t.Setenv(EnvConfigLegacy, "")
	t.Setenv(EnvRunpodAPIKey, "")
	if err := SaveHFToken("stored-hf"); err != nil {
		t.Fatal(err)
	}
	t.Setenv(EnvHFToken, "env-hf")
	if Load().HFToken != "env-hf" {
		t.Fatalf("env first: %q", Load().HFToken)
	}
	t.Setenv(EnvHFToken, "")
	if Load().HFToken != "stored-hf" {
		t.Fatalf("stored: %q", Load().HFToken)
	}
}

func TestSaveHFTokenPermissionsAndDelete(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(EnvConfig, filepath.Join(dir, "registry.json"))
	t.Setenv(EnvHFToken, "")
	if err := SaveHFToken("  hf-secret  "); err != nil {
		t.Fatal(err)
	}
	path, err := StoredHFTokenPath()
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
	if filepath.Base(path) != "hf.token" {
		t.Fatalf("path %q", path)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(raw) != "hf-secret\n" {
		t.Fatalf("file %q", raw)
	}
	if Load().HFToken != "hf-secret" {
		t.Fatal("load after save")
	}
	if err := DeleteHFToken(); err != nil {
		t.Fatal(err)
	}
	if Load().HFToken != "" {
		t.Fatal("expected empty after delete")
	}
	if err := DeleteHFToken(); err != nil {
		t.Fatal(err)
	}
}

func TestSaveHFTokenRejectsEmpty(t *testing.T) {
	t.Setenv(EnvConfig, filepath.Join(t.TempDir(), "registry.json"))
	if err := SaveHFToken("  "); err == nil {
		t.Fatal("expected error")
	}
}

func TestSettingsLoadSaveNoColor(t *testing.T) {
	dir := t.TempDir()
	t.Setenv(EnvConfig, filepath.Join(dir, "registry.json"))
	t.Setenv("NO_COLOR", "")
	if LoadSettings().NoColor {
		t.Fatal("default false")
	}
	if err := SaveSettings(Settings{NoColor: true}); err != nil {
		t.Fatal(err)
	}
	path, err := SettingsPath()
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(path) != "settings.json" {
		t.Fatalf("path %q", path)
	}
	st, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if st.Mode().Perm() != 0o600 {
		t.Fatalf("perm %o", st.Mode().Perm())
	}
	s := LoadSettings()
	if !s.NoColor {
		t.Fatal("expected no_color true")
	}
	if !ColorDisabled() {
		t.Fatal("ColorDisabled")
	}
	if err := SaveSettings(Settings{NoColor: false}); err != nil {
		t.Fatal(err)
	}
	if LoadSettings().NoColor {
		t.Fatal("expected false")
	}
}
