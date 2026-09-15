package cli

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/adamsiwiec1/runhug/internal/config"
	"github.com/adamsiwiec1/runhug/internal/hf"
	"github.com/adamsiwiec1/runhug/internal/runpod"
)

// runpodAPIKeysURL is the official console page for creating or copying API keys.
// Runpod's public REST API is Bearer-key only. Flash login and hosted MCP "Sign
// in with Runpod" are product-specific; this CLI does not impersonate them.
const runpodAPIKeysURL = "https://console.runpod.io/user/credentials?tab=api-key"
const hfTokensURL = "https://huggingface.co/settings/tokens"

var (
	verifyKey  = verifyRunpodKey
	verifyHF   = verifyHFToken
	promptOK   = canPrompt
	askReplace = func() (bool, error) {
		return confirmPrefErr("Replace the stored key?", false)
	}
	askReplaceHF = func() (bool, error) {
		return confirmPrefErr("Replace the stored Hugging Face token?", false)
	}
	readAPIKey = func() (string, error) {
		return readSecret("Runpod API key: ")
	}
	readHFToken = func() (string, error) {
		return readSecret("Hugging Face token: ")
	}
)

func cmdConnect(args []string) error {
	if len(args) > 0 {
		switch strings.ToLower(args[0]) {
		case "hf", "huggingface", "hub":
			return cmdConnectHF(args[1:])
		}
	}
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
		"runhug list",
		"runhug gpus",
		"runhug deploy Qwen/Qwen2.5-7B-Instruct",
		"runhug proxy",
	)
}

func cmdDisconnect(args []string) error {
	if len(args) > 0 {
		switch strings.ToLower(args[0]) {
		case "hf", "huggingface", "hub":
			return cmdDisconnectHF(args[1:])
		}
	}
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

func cmdLogin(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: runhug login hf")
	}
	switch strings.ToLower(args[0]) {
	case "hf", "huggingface", "hub":
		return cmdConnectHF(args[1:])
	default:
		return fmt.Errorf("unknown login target %q — try `login hf`", args[0])
	}
}

func cmdHF(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("usage: runhug hf login | runhug hf disconnect")
	}
	switch strings.ToLower(args[0]) {
	case "login", "connect":
		return cmdConnectHF(args[1:])
	case "disconnect", "logout":
		return cmdDisconnectHF(args[1:])
	default:
		return fmt.Errorf("unknown hf command %q — try `hf login` or `hf disconnect`", args[0])
	}
}

func cmdConnectHF(args []string) error {
	fs := newFlagSet("connect hf")
	tokenFlag := fs.String("token", "", "Hugging Face token (otherwise a hidden prompt)")
	keyFlag := fs.String("key", "", "alias for --token")
	if err := parseFlags(fs, args); err != nil {
		return err
	}

	heading(os.Stdout, "Connect Hugging Face")
	tokenSet := false
	fs.Visit(func(f *flag.Flag) {
		if f.Name == "token" || f.Name == "key" {
			tokenSet = true
		}
	})
	token := config.SanitizeAPIKey(*tokenFlag)
	if token == "" {
		token = config.SanitizeAPIKey(*keyFlag)
	}
	if tokenSet {
		return saveVerifiedHFToken(token, flagSourceHF(fs))
	}

	viaEnv := config.SanitizeAPIKey(os.Getenv(config.EnvHFToken)) != ""
	stored := config.HasStoredHFToken()
	if viaEnv || stored {
		return connectHFAlready(viaEnv, stored)
	}

	printHFConnectIntro(os.Stdout)
	if !promptOK() {
		return fmt.Errorf("pass --token or set %s, or run `connect hf` in a terminal", config.EnvHFToken)
	}
	line, err := readHFToken()
	if err != nil {
		return err
	}
	return saveVerifiedHFToken(line, "prompt")
}

func flagSourceHF(fs *flag.FlagSet) string {
	name := "prompt"
	fs.Visit(func(f *flag.Flag) {
		if f.Name == "token" {
			name = "--token"
		} else if f.Name == "key" && name == "prompt" {
			name = "--key"
		}
	})
	return name
}

func connectHFAlready(viaEnv, stored bool) error {
	printAlreadyConnectedHF(os.Stdout, viaEnv, stored)
	fmt.Fprintln(os.Stdout, cyan(hfTokensURL))
	fmt.Fprintln(os.Stdout)
	if !promptOK() {
		fmt.Fprintln(os.Stdout, dim("To replace the stored token, pass --token or run in a terminal."))
		fmt.Fprintln(os.Stdout)
		return nil
	}
	ok, err := askReplaceHF()
	if err != nil {
		return err
	}
	if !ok {
		fmt.Fprintln(os.Stdout, "Kept the existing token.")
		fmt.Fprintln(os.Stdout)
		return nil
	}
	printHFConnectIntro(os.Stdout)
	line, err := readHFToken()
	if err != nil {
		return err
	}
	return saveVerifiedHFToken(line, "prompt")
}

func saveVerifiedHFToken(token, from string) error {
	token = config.SanitizeAPIKey(token)
	if token == "" {
		return fmt.Errorf("empty Hugging Face token")
	}
	name, err := verifyHF(token)
	if err != nil {
		if isUnauthorized(err) {
			return fmt.Errorf("Hugging Face rejected this token")
		}
		return fmt.Errorf("could not verify with Hugging Face: %w", err)
	}
	if err := config.SaveHFToken(token); err != nil {
		return err
	}
	path, err := config.StoredHFTokenPath()
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stdout, "%s  Hugging Face", green("Connected"))
	if name != "" {
		fmt.Fprintf(os.Stdout, "  (%s)", name)
	}
	fmt.Fprintln(os.Stdout)
	printKV(os.Stdout, "saved", path+"  (0600, never printed)")
	printKV(os.Stdout, "source", from)
	if from != config.EnvHFToken && config.SanitizeAPIKey(os.Getenv(config.EnvHFToken)) != "" {
		fmt.Fprintln(os.Stdout)
		fmt.Fprintln(os.Stdout, yellow(config.EnvHFToken+" is set and still wins over the stored token (not printed)."))
	}
	fmt.Fprintln(os.Stdout)
	return nil
}

func printAlreadyConnectedHF(w io.Writer, viaEnv, stored bool) {
	switch {
	case viaEnv && stored:
		fmt.Fprintf(w, "%s  Hugging Face (%s wins; stored token kept, never printed)\n", green("Connected"), config.EnvHFToken)
	case viaEnv:
		fmt.Fprintf(w, "%s  Hugging Face via %s (never printed)\n", green("Connected"), config.EnvHFToken)
	default:
		fmt.Fprintf(w, "%s  Hugging Face via stored token (never printed)\n", green("Connected"))
	}
	if stored {
		if path, err := config.StoredHFTokenPath(); err == nil {
			printKV(w, "saved", path+"  (0600)")
		}
	}
	fmt.Fprintln(w)
}

func printHFConnectIntro(w io.Writer) {
	fmt.Fprintln(w, "Create or copy a token on Hugging Face, then paste it here.")
	fmt.Fprintln(w, dim("It is saved locally (mode 0600) and never printed."))
	fmt.Fprintln(w)
	fmt.Fprintln(w, cyan(hfTokensURL))
	fmt.Fprintln(w)
	fmt.Fprintln(w, dim("Open that URL, then paste a token here."))
	fmt.Fprintln(w)
}

func cmdDisconnectHF(args []string) error {
	fs := newFlagSet("disconnect hf")
	if err := parseFlags(fs, args); err != nil {
		return err
	}
	had := config.HasStoredHFToken()
	if err := config.DeleteHFToken(); err != nil {
		return err
	}
	heading(os.Stdout, "Disconnect Hugging Face")
	if !had {
		fmt.Fprintln(os.Stdout, "No stored Hugging Face token.")
		if config.SanitizeAPIKey(os.Getenv(config.EnvHFToken)) != "" {
			fmt.Fprintln(os.Stdout, yellow(config.EnvHFToken+" is still set in this environment (not printed)."))
		}
		fmt.Fprintln(os.Stdout)
		return nil
	}
	fmt.Fprintf(os.Stdout, "%s  stored Hugging Face token removed\n\n", green("Disconnected"))
	return nil
}

func verifyHFToken(token string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	return hf.New(token).Whoami(ctx)
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
