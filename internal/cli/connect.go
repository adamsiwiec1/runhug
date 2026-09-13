package cli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/adamsiwiec1/runpod-vllm-proxy/internal/config"
	"github.com/adamsiwiec1/runpod-vllm-proxy/internal/runpod"
)

// runpodAPIKeysURL is the official console page for creating or copying API keys.
// Runpod's public REST API is Bearer-key only. Flash login and hosted MCP "Sign
// in with Runpod" are product-specific; this CLI does not impersonate them.
const runpodAPIKeysURL = "https://console.runpod.io/user/credentials?tab=api-key"

var (
	verifyKey  = verifyRunpodKey
	promptOK   = canPrompt
	askReplace = func() (bool, error) {
		return confirmPrefErr("Replace the stored key?", false)
	}
	readAPIKey = func() (string, error) {
		return readSecret("Runpod API key: ")
	}
)

func cmdConnect(args []string) error {
	fs := newFlagSet("connect")
	keyFlag := fs.String("key", "", "Runpod API key (otherwise a hidden prompt)")
	if err := parseFlags(fs, args); err != nil {
		return err
	}

	heading(os.Stdout, "Connect")
	keySet := false
	fs.Visit(func(f *flag.Flag) {
		if f.Name == "key" {
			keySet = true
		}
	})
	key := config.SanitizeAPIKey(*keyFlag)
	if keySet {
		return saveVerifiedKey(key, "--key")
	}

	viaEnv := config.SanitizeAPIKey(os.Getenv(config.EnvRunpodAPIKey)) != ""
	stored := config.HasStoredKey()
	if viaEnv || stored {
		return connectAlready(viaEnv, stored)
	}

	printConnectIntro(os.Stdout)
	if !promptOK() {
		return fmt.Errorf("pass --key or set %s, or run `connect` in a terminal", config.EnvRunpodAPIKey)
	}
	line, err := readAPIKey()
	if err != nil {
		return err
	}
	return saveVerifiedKey(line, "prompt")
}

func connectAlready(viaEnv, stored bool) error {
	printAlreadyConnected(os.Stdout, viaEnv, stored)
	printConsoleURL(os.Stdout)
	fmt.Fprintln(os.Stdout)
	if !promptOK() {
		fmt.Fprintln(os.Stdout, dim("To replace the stored key, pass --key or run in a terminal."))
		fmt.Fprintln(os.Stdout)
		printConnectNext()
		return nil
	}
	ok, err := askReplace()
	if err != nil {
		return err
	}
	if !ok {
		fmt.Fprintln(os.Stdout, "Kept the existing key.")
		fmt.Fprintln(os.Stdout)
		printConnectNext()
		return nil
	}
	printConnectIntro(os.Stdout)
	line, err := readAPIKey()
	if err != nil {
		return err
	}
	return saveVerifiedKey(line, "prompt")
}

func saveVerifiedKey(key, from string) error {
	key = config.SanitizeAPIKey(key)
	if key == "" {
		return fmt.Errorf("empty API key")
	}
	if err := verifyKey(key); err != nil {
		if isUnauthorized(err) {
			return fmt.Errorf("Runpod rejected this API key")
		}
		return fmt.Errorf("could not verify with Runpod: %w", err)
	}
	if err := config.SaveKey(key); err != nil {
		return err
	}
	path, err := config.StoredKeyPath()
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stdout, "%s  Runpod\n", green("Connected"))
	printKV(os.Stdout, "saved", path+"  (0600, never printed)")
	printKV(os.Stdout, "source", from)
	if from != config.EnvRunpodAPIKey && config.SanitizeAPIKey(os.Getenv(config.EnvRunpodAPIKey)) != "" {
		fmt.Fprintln(os.Stdout)
		fmt.Fprintln(os.Stdout, yellow(config.EnvRunpodAPIKey+" is set and still wins over the stored key (not printed)."))
	}
	fmt.Fprintln(os.Stdout)
	printConnectNext()
	return nil
}

func printAlreadyConnected(w io.Writer, viaEnv, stored bool) {
	switch {
	case viaEnv && stored:
		fmt.Fprintf(w, "%s  Runpod (%s wins; stored key kept, never printed)\n", green("Connected"), config.EnvRunpodAPIKey)
	case viaEnv:
		fmt.Fprintf(w, "%s  Runpod via %s (never printed)\n", green("Connected"), config.EnvRunpodAPIKey)
	default:
		fmt.Fprintf(w, "%s  Runpod via stored key (never printed)\n", green("Connected"))
	}
	if stored {
		if path, err := config.StoredKeyPath(); err == nil {
			printKV(w, "saved", path+"  (0600)")
		}
	}
	fmt.Fprintln(w)
}

func printConnectIntro(w io.Writer) {
	fmt.Fprintln(w, "Create or copy an API key in the Runpod console, then paste it here.")
	fmt.Fprintln(w, dim("It is saved locally (mode 0600) and never printed. There is no OAuth for this CLI."))
	fmt.Fprintln(w)
	printConsoleURL(w)
	fmt.Fprintln(w)
	fmt.Fprintln(w, dim("Open that URL, then paste a key here."))
	fmt.Fprintln(w)
}

func printConsoleURL(w io.Writer) {
	fmt.Fprintln(w, cyan(runpodAPIKeysURL))
}

func printConnectNext() {
	commands(os.Stdout, "Next:",
		"runpod-vllm-proxy list",
		"runpod-vllm-proxy gpus",
		"runpod-vllm-proxy deploy Qwen/Qwen2.5-7B-Instruct",
		"runpod-vllm-proxy proxy",
	)
}

func cmdDisconnect(args []string) error {
	fs := newFlagSet("disconnect")
	if err := parseFlags(fs, args); err != nil {
		return err
	}
	had := config.HasStoredKey()
	if err := config.DeleteKey(); err != nil {
		return err
	}
	heading(os.Stdout, "Disconnect")
	if !had {
		fmt.Fprintln(os.Stdout, "No stored key.")
		if config.SanitizeAPIKey(os.Getenv(config.EnvRunpodAPIKey)) != "" {
			fmt.Fprintln(os.Stdout, yellow(config.EnvRunpodAPIKey+" is still set in this environment (not printed)."))
		}
		fmt.Fprintln(os.Stdout)
		return nil
	}
	fmt.Fprintf(os.Stdout, "%s  stored key removed\n\n", green("Disconnected"))
	return nil
}

func verifyRunpodKey(key string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	_, err := runpod.New(key).ListEndpoints(ctx, 1)
	return err
}

func isUnauthorized(err error) bool {
	if err == nil {
		return false
	}
	s := strings.ToLower(err.Error())
	return strings.Contains(s, "401") || strings.Contains(s, "unauthorized")
}
