package cli

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/adamsiwiec1/runhug-cli/internal/config"
)

func cmdConfig(args []string) error {
	if len(args) == 0 {
		return printConfigInfo()
	}
	switch strings.ToLower(args[0]) {
	case "get":
		return cmdConfigGet(args[1:])
	case "set":
		return cmdConfigSet(args[1:])
	case "path", "paths", "show":
		return printConfigInfo()
	default:
		return fmt.Errorf("usage: runhug-cli config [get|set] <key> [value]\nKnown keys: no_color, update_limit, advisor_base_url, advisor_model")
	}
}

func printConfigInfo() error {
	heading(os.Stdout, "Config")
	dir, err := config.Dir()
	if err != nil {
		return err
	}
	printKV(os.Stdout, "dir", dir)
	if override := config.ConfigPathOverride(); override != "" {
		printKV(os.Stdout, "override", override+"  (RUNHUG_CONFIG / RVP_CONFIG)")
	}
	if p, err := config.StoredKeyPath(); err == nil {
		status := "missing"
		if config.HasStoredKey() {
			status = "present (0600, never printed)"
		}
		printKV(os.Stdout, "runpod", p+"  — "+status)
	}
	if p, err := config.StoredHFTokenPath(); err == nil {
		status := "missing"
		if config.HasStoredHFToken() {
			status = "present (0600, never printed)"
		}
		printKV(os.Stdout, "hf.token", p+"  — "+status)
	}
	if p, err := config.SettingsPath(); err == nil {
		printKV(os.Stdout, "settings", p)
	}
	s := config.LoadSettings()
	printKV(os.Stdout, "no_color", strconv.FormatBool(s.NoColor || config.ColorDisabled()))
	if s.NoColor {
		printKV(os.Stdout, "source", "settings.json")
	} else if os.Getenv("NO_COLOR") != "" {
		printKV(os.Stdout, "source", "NO_COLOR env")
	} else {
		printKV(os.Stdout, "source", "default (colors when TTY)")
	}
	printKV(os.Stdout, "update_limit", strconv.Itoa(config.EffectiveUpdateLimit())+"  (0=unlimited; flag>env>settings>2000)")
	if s.AdvisorBaseURL != "" {
		printKV(os.Stdout, "advisor_base_url", s.AdvisorBaseURL)
	} else {
		printKV(os.Stdout, "advisor_base_url", "http://127.0.0.1:11434/v1  (default Ollama)")
	}
	if s.AdvisorModel != "" {
		printKV(os.Stdout, "advisor_model", s.AdvisorModel)
	} else {
		printKV(os.Stdout, "advisor_model", "(unset — pass --model or set advisor_model)")
	}
	fmt.Fprintln(os.Stdout)
	commands(os.Stdout, "Examples:",
		"runhug-cli config set no_color true",
		"runhug-cli config set update_limit 5000",
		"runhug-cli config set advisor_base_url http://127.0.0.1:11434/v1",
		"runhug-cli config get update_limit",
		"runhug-cli connect hf",
	)
	return nil
}

func cmdConfigGet(args []string) error {
	if len(args) == 0 {
		return printConfigInfo()
	}
	key := strings.ToLower(strings.TrimSpace(args[0]))
	s := config.LoadSettings()
	switch key {
	case "no_color", "nocolor":
		fmt.Println(strconv.FormatBool(s.NoColor))
		return nil
	case "update_limit", "updatelimit":
		fmt.Println(config.EffectiveUpdateLimit())
		return nil
	case "advisor_base_url", "advisor_url":
		if s.AdvisorBaseURL == "" {
			fmt.Println("http://127.0.0.1:11434/v1")
		} else {
			fmt.Println(s.AdvisorBaseURL)
		}
		return nil
	case "advisor_model":
		fmt.Println(s.AdvisorModel)
		return nil
	case "dir", "path":
		dir, err := config.Dir()
		if err != nil {
			return err
		}
		fmt.Println(dir)
		return nil
	default:
		return fmt.Errorf("unknown setting %q — known: no_color, update_limit, advisor_base_url, advisor_model, dir", args[0])
	}
}

func cmdConfigSet(args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: runhug-cli config set <key> <value>")
	}
	key := strings.ToLower(strings.TrimSpace(args[0]))
	val := strings.TrimSpace(args[1])
	s := config.LoadSettings()
	switch key {
	case "no_color", "nocolor":
		b, err := parseBoolArg(strings.ToLower(val))
		if err != nil {
			return err
		}
		s.NoColor = b
	case "update_limit", "updatelimit":
		n, err := strconv.Atoi(val)
		if err != nil || n < 0 {
			return fmt.Errorf("update_limit must be an integer >= 0 (0=unlimited)")
		}
		s.UpdateLimit = &n
	case "advisor_base_url", "advisor_url":
		s.AdvisorBaseURL = strings.TrimRight(val, "/")
	case "advisor_model":
		s.AdvisorModel = val
	default:
		return fmt.Errorf("unknown setting %q — known: no_color, update_limit, advisor_base_url, advisor_model", args[0])
	}
	if err := config.SaveSettings(s); err != nil {
		return err
	}
	path, _ := config.SettingsPath()
	fmt.Fprintf(os.Stdout, "%s  %s=%s\n", green("Saved"), key, val)
	if path != "" {
		printKV(os.Stdout, "file", path)
	}
	fmt.Fprintln(os.Stdout)
	return nil
}

func parseBoolArg(s string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "1", "true", "yes", "on":
		return true, nil
	case "0", "false", "no", "off":
		return false, nil
	default:
		return false, fmt.Errorf("expected true|false, got %q", s)
	}
}
