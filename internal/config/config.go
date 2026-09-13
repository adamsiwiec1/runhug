package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	EnvRunpodAPIKey = "RUNPOD_API_KEY"
	EnvHFToken      = "HF_TOKEN"
	storedKeyName   = "runpod.key"
)

type Env struct {
	RunpodAPIKey string
	HFToken      string
}

// SanitizeAPIKey trims space, strips ASCII controls (including CR/LF), and
// drops a leading "Bearer " so a pasted or stored key is a valid HTTP token.
// The value is never logged. An empty result means reject the key.
func SanitizeAPIKey(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c < 32 || c == 127 {
			continue
		}
		b.WriteByte(c)
	}
	s = strings.TrimSpace(b.String())
	const bearer = "bearer "
	if len(s) >= len(bearer) && strings.EqualFold(s[:len(bearer)], bearer) {
		s = strings.TrimSpace(s[len(bearer):])
	}
	return s
}

func Load() Env {
	key := SanitizeAPIKey(os.Getenv(EnvRunpodAPIKey))
	if key == "" {
		key = loadStoredKey()
	}
	return Env{
		RunpodAPIKey: key,
		HFToken:      strings.TrimSpace(os.Getenv(EnvHFToken)),
	}
}

func (e Env) RequireRunpod() error {
	if e.RunpodAPIKey == "" {
		return fmt.Errorf("not connected — run `runpod-vllm-proxy connect` or set %s", EnvRunpodAPIKey)
	}
	return nil
}

func (e Env) Connected() bool {
	return e.RunpodAPIKey != ""
}

func Dir() (string, error) {
	if override := strings.TrimSpace(os.Getenv("RVP_CONFIG")); override != "" {
		return filepath.Dir(override), nil
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "runpod-vllm-proxy"), nil
}

func StoredKeyPath() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, storedKeyName), nil
}

func SaveKey(key string) error {
	key = SanitizeAPIKey(key)
	if key == "" {
		return fmt.Errorf("empty API key")
	}
	path, err := StoredKeyPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	if err := os.WriteFile(path, []byte(key+"\n"), 0o600); err != nil {
		return err
	}
	return os.Chmod(path, 0o600)
}

func DeleteKey() error {
	path, err := StoredKeyPath()
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func HasStoredKey() bool {
	return loadStoredKey() != ""
}

func loadStoredKey() string {
	path, err := StoredKeyPath()
	if err != nil {
		return ""
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return SanitizeAPIKey(string(raw))
}
