package cli

import (
	"bytes"
	"os"
	"strings"
	"testing"
	"time"
)

func TestWizardAliasesRegistered(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	for _, cmd := range []string{"wizard", "guide", "guided", "setup"} {
		cmd := cmd
		t.Run(cmd, func(t *testing.T) {
			done := make(chan error, 1)
			go func() {
				devNull, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
				if err != nil {
					done <- err
					return
				}
				defer devNull.Close()
				stdout := os.Stdout
				os.Stdout = devNull
				defer func() { os.Stdout = stdout }()
				done <- Run([]string{cmd, "--yes"})
			}()
			select {
			case err := <-done:
				if err != nil {
					t.Fatalf("%s --yes: %v", cmd, err)
				}
			case <-time.After(3 * time.Second):
				t.Fatalf("%s --yes hung (expected non-interactive checklist)", cmd)
			}
		})
	}
}

func TestWizardChecklistContent(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	var buf bytes.Buffer
	if err := wizardChecklist(&buf); err != nil {
		t.Fatal(err)
	}
	s := buf.String()
	for _, want := range []string{
		"Wizard",
		"Init search stack",
		"HF token",
		"Runpod",
		"Advisor",
		"Find a model",
		"GPU",
		"dry-run",
		"Live deploy",
		"Proxy",
		"Never auto-creates a live Runpod endpoint",
		"runhug-cli init",
		"runhug-cli deploy <org/model> --dry-run",
	} {
		if !strings.Contains(s, want) {
			t.Fatalf("checklist missing %q\n%s", want, s)
		}
	}
}

func TestWizardYesDoesNotReadStdin(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	_ = w.Close()
	oldIn := os.Stdin
	os.Stdin = r
	defer func() {
		os.Stdin = oldIn
		_ = r.Close()
	}()

	done := make(chan error, 1)
	go func() {
		devNull, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
		if err != nil {
			done <- err
			return
		}
		defer devNull.Close()
		stdout := os.Stdout
		os.Stdout = devNull
		defer func() { os.Stdout = stdout }()
		done <- cmdWizard([]string{"--yes"})
	}()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("cmdWizard --yes hung with EOF stdin")
	}
}

func TestUsageMentionsWizard(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	var buf bytes.Buffer
	printUsage(&buf)
	s := buf.String()
	if !strings.Contains(s, "wizard") {
		t.Fatalf("usage missing wizard\n%s", s)
	}
	if !strings.Contains(s, "guide") {
		t.Fatalf("usage missing guide alias mention\n%s", s)
	}
}
