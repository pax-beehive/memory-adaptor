package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/pax-beehive/memory-adaptor/internal/adapters"
	"github.com/pax-beehive/memory-adaptor/internal/config"
	"github.com/pax-beehive/memory-adaptor/internal/facade"
)

type runner struct {
	stdin      io.Reader
	stdout     io.Writer
	stderr     io.Writer
	configPath string
}

func Main(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if stdin == nil {
		stdin = strings.NewReader("")
	}
	if stdout == nil {
		stdout = io.Discard
	}
	if stderr == nil {
		stderr = io.Discard
	}
	args, configPath, err := extractConfigFlag(args)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	r := runner{
		stdin:      stdin,
		stdout:     stdout,
		stderr:     stderr,
		configPath: configPath,
	}
	if len(args) == 0 {
		r.printHelp()
		return 0
	}
	if args[0] == "help" || args[0] == "--help" || args[0] == "-h" {
		r.printHelp()
		return 0
	}
	if err := r.run(args); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	return 0
}

func (r runner) run(args []string) error {
	switch args[0] {
	case "setup":
		return r.runSetup(args[1:])
	case "recall":
		return r.runRecall(args[1:])
	case "remember":
		return r.runRemember(args[1:])
	case "config":
		return r.runConfig(args[1:])
	default:
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func (r runner) runSetup(args []string) error {
	fs := flag.NewFlagSet("setup", flag.ContinueOnError)
	fs.SetOutput(r.stderr)
	force := fs.Bool("force", false, "overwrite an existing config")
	yes := fs.Bool("yes", false, "accept default setup answers")
	if err := fs.Parse(args); err != nil {
		return err
	}

	path := r.configFile()
	promptReader := bufio.NewReader(r.stdin)
	if config.Exists(path) && !*force {
		if *yes {
			return fmt.Errorf("config already exists at %s; use --force to overwrite", path)
		}
		overwrite, err := promptBool(promptReader, r.stdout, fmt.Sprintf("Config already exists at %s. Overwrite?", path), false)
		if err != nil {
			return err
		}
		if !overwrite {
			fmt.Fprintln(r.stdout, "setup cancelled")
			return nil
		}
	}
	cfg := config.DefaultConfig(path)
	local := cfg.Providers["local"]
	enableLocal := true
	installCodexHook := true
	if !*yes {
		var err error
		enableLocal, err = promptBool(promptReader, r.stdout, "Enable local memory provider?", true)
		if err != nil {
			return err
		}
		if enableLocal {
			local.Path, err = promptString(promptReader, r.stdout, "Local memory path", local.Path)
			if err != nil {
				return err
			}
			local.Read, err = promptBool(promptReader, r.stdout, "Read from local provider?", true)
			if err != nil {
				return err
			}
			local.Write, err = promptBool(promptReader, r.stdout, "Write to local provider?", true)
			if err != nil {
				return err
			}
			local.Required, err = promptBool(promptReader, r.stdout, "Require local provider to succeed?", true)
			if err != nil {
				return err
			}
		}
		installCodexHook, err = promptBool(promptReader, r.stdout, "Install Codex passive recall hook shim?", true)
		if err != nil {
			return err
		}
	}
	local.Enabled = enableLocal
	cfg.Providers["local"] = local
	codexHook := cfg.Hooks["codex"]
	codexHook.Enabled = installCodexHook
	if eventCfg, ok := codexHook.Events["user_prompt"]; ok {
		eventCfg.Recall.Enabled = installCodexHook
		codexHook.Events["user_prompt"] = eventCfg
	}
	cfg.Hooks["codex"] = codexHook
	if err := config.Save(path, cfg); err != nil {
		return err
	}
	fmt.Fprintf(r.stdout, "created config: %s\n", path)
	if installCodexHook {
		scriptPath, err := installHookShim(path, "codex", "user_prompt")
		if err != nil {
			return err
		}
		fmt.Fprintf(r.stdout, "installed hook shim: %s\n", scriptPath)
	}
	return nil
}

func (r runner) runRecall(args []string) error {
	fs := flag.NewFlagSet("recall", flag.ContinueOnError)
	fs.SetOutput(r.stderr)
	query := fs.String("query", "", "recall query")
	queryShort := fs.String("q", "", "recall query")
	limit := fs.Int("limit", 8, "maximum memories to return")
	jsonOut := fs.Bool("json", false, "write JSON")
	stdin := fs.Bool("stdin", false, "read query from stdin")
	hookEvent := fs.Bool("hook-event", false, "read a hook event from stdin")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *hookEvent {
		var event facade.HookEvent
		bytes, err := io.ReadAll(r.stdin)
		if err != nil {
			return err
		}
		if err := json.Unmarshal(bytes, &event); err != nil {
			return fmt.Errorf("decode hook event JSON: %w", err)
		}
		return r.executeHook(event, *jsonOut)
	}
	q := firstNonEmpty(*query, *queryShort)
	if *stdin {
		bytes, err := io.ReadAll(r.stdin)
		if err != nil {
			return err
		}
		q = string(bytes)
	}

	service, err := r.loadService()
	if err != nil {
		return err
	}
	result, err := service.Recall(context.Background(), facade.RecallInput{
		Query: q,
		Limit: *limit,
	})
	if err != nil {
		return err
	}
	if *jsonOut {
		return writeJSON(r.stdout, result)
	}
	writeRecallMarkdown(r.stdout, result)
	return nil
}

func (r runner) runRemember(args []string) error {
	fs := flag.NewFlagSet("remember", flag.ContinueOnError)
	fs.SetOutput(r.stderr)
	text := fs.String("text", "", "memory text")
	source := fs.String("source", "cli", "memory source")
	jsonOut := fs.Bool("json", false, "write JSON")
	stdin := fs.Bool("stdin", false, "read memory text from stdin")
	if err := fs.Parse(args); err != nil {
		return err
	}
	value := *text
	if *stdin {
		bytes, err := io.ReadAll(r.stdin)
		if err != nil {
			return err
		}
		value = string(bytes)
	}

	service, err := r.loadService()
	if err != nil {
		return err
	}
	result, err := service.Ingest(context.Background(), facade.IngestInput{
		Text:   value,
		Source: *source,
	})
	if err != nil {
		return err
	}
	if *jsonOut {
		return writeJSON(r.stdout, result)
	}
	for _, ref := range result.Refs {
		fmt.Fprintf(r.stdout, "stored memory: %s/%s\n", ref.Provider, ref.ID)
	}
	return nil
}

func (r runner) executeHook(event facade.HookEvent, jsonOut bool) error {
	service, err := r.loadService()
	if err != nil {
		return err
	}
	result, err := service.RunHook(context.Background(), event)
	if err != nil {
		return err
	}
	if jsonOut {
		return writeJSON(r.stdout, result)
	}
	if result.Skipped || result.Recall == nil {
		return nil
	}
	writeRecallMarkdown(r.stdout, *result.Recall)
	return nil
}

func (r runner) runConfig(args []string) error {
	if len(args) == 0 {
		return errors.New("config command requires a subcommand: path, show, doctor")
	}
	switch args[0] {
	case "path":
		fmt.Fprintln(r.stdout, r.configFile())
		return nil
	case "show":
		cfg, err := config.Load(r.configFile())
		if err != nil {
			return err
		}
		return writeJSON(r.stdout, cfg)
	case "doctor":
		return r.runConfigDoctor(args[1:])
	default:
		return fmt.Errorf("unknown config subcommand %q", args[0])
	}
}

func (r runner) runConfigDoctor(args []string) error {
	fs := flag.NewFlagSet("config doctor", flag.ContinueOnError)
	fs.SetOutput(r.stderr)
	jsonOut := fs.Bool("json", false, "write JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	cfg, err := config.Load(r.configFile())
	if err != nil {
		return err
	}
	router, err := adapters.DefaultRegistry().BuildRouter(cfg)
	if err != nil {
		return err
	}
	statuses, err := router.Health(context.Background())
	if *jsonOut {
		if writeErr := writeJSON(r.stdout, statuses); writeErr != nil {
			return writeErr
		}
		return err
	}
	for _, status := range statuses {
		if status.OK {
			fmt.Fprintf(r.stdout, "ok: %s\n", status.Provider)
			continue
		}
		fmt.Fprintf(r.stdout, "error: %s: %s\n", status.Provider, status.Error)
	}
	return err
}

func (r runner) loadService() (*facade.Service, error) {
	cfg, err := config.Load(r.configFile())
	if err != nil {
		if errors.Is(err, config.ErrConfigMissing) {
			return nil, fmt.Errorf("%w; run `paxm --config %s setup`", err, r.configFile())
		}
		return nil, err
	}
	router, err := adapters.DefaultRegistry().BuildRouter(cfg)
	if err != nil {
		return nil, err
	}
	return facade.New(cfg, router), nil
}

func (r runner) configFile() string {
	if r.configPath != "" {
		return config.ExpandPath(r.configPath)
	}
	return config.DefaultConfigPath()
}

func (r runner) printHelp() {
	fmt.Fprintln(r.stdout, "paxm - memory adapter CLI")
	fmt.Fprintln(r.stdout)
	fmt.Fprintln(r.stdout, "Usage:")
	fmt.Fprintln(r.stdout, "  paxm [--config PATH] setup")
	fmt.Fprintln(r.stdout, "  paxm [--config PATH] recall --query TEXT [--json]")
	fmt.Fprintln(r.stdout, "  paxm [--config PATH] remember --text TEXT")
	fmt.Fprintln(r.stdout, "  paxm [--config PATH] config doctor")
}

func promptBool(reader *bufio.Reader, writer io.Writer, question string, defaultValue bool) (bool, error) {
	suffix := " [y/N]: "
	if defaultValue {
		suffix = " [Y/n]: "
	}
	for {
		fmt.Fprint(writer, question+suffix)
		line, err := reader.ReadString('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			return false, err
		}
		value := strings.ToLower(strings.TrimSpace(line))
		if value == "" {
			return defaultValue, nil
		}
		switch value {
		case "y", "yes":
			return true, nil
		case "n", "no":
			return false, nil
		default:
			fmt.Fprintln(writer, "Please answer yes or no.")
		}
		if errors.Is(err, io.EOF) {
			return defaultValue, nil
		}
	}
}

func promptString(reader *bufio.Reader, writer io.Writer, question, defaultValue string) (string, error) {
	fmt.Fprintf(writer, "%s [%s]: ", question, defaultValue)
	line, err := reader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}
	value := strings.TrimSpace(line)
	if value == "" {
		return defaultValue, nil
	}
	return value, nil
}

func installHookShim(configPath, target, event string) (string, error) {
	hooksDir := filepath.Join(filepath.Dir(config.ExpandPath(configPath)), "hooks")
	if err := os.MkdirAll(hooksDir, 0o755); err != nil {
		return "", err
	}
	binaryPath, err := os.Executable()
	if err != nil || binaryPath == "" {
		binaryPath = "paxm"
	}
	scriptPath := filepath.Join(hooksDir, target+"-"+event)
	script := "#!/bin/sh\nexec " + shellQuote(binaryPath) + " --config " + shellQuote(config.ExpandPath(configPath)) + " recall --hook-event --json\n"
	if err := os.WriteFile(scriptPath, []byte(script), 0o755); err != nil {
		return "", err
	}
	return scriptPath, nil
}

func shellQuote(value string) string {
	if value == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(value, "'", "'\"'\"'") + "'"
}

func extractConfigFlag(args []string) ([]string, string, error) {
	var filtered []string
	var configPath string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--config" || arg == "-c":
			if i+1 >= len(args) {
				return nil, "", fmt.Errorf("%s requires a path", arg)
			}
			configPath = args[i+1]
			i++
		case strings.HasPrefix(arg, "--config="):
			configPath = strings.TrimPrefix(arg, "--config=")
		default:
			filtered = append(filtered, arg)
		}
	}
	return filtered, configPath, nil
}

func writeJSON(w io.Writer, value any) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}

func writeRecallMarkdown(w io.Writer, result facade.RecallResult) {
	if len(result.Hits) == 0 {
		fmt.Fprintln(w, "No memories found.")
		return
	}
	for i, hit := range result.Hits {
		fmt.Fprintf(w, "### Memory %d (%s)\n", i+1, hit.Provider)
		if hit.Source != "" {
			fmt.Fprintf(w, "Source: %s\n\n", hit.Source)
		} else {
			fmt.Fprintln(w)
		}
		fmt.Fprintln(w, strings.TrimSpace(hit.Text))
		fmt.Fprintln(w)
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
