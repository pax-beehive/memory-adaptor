package cli

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"sort"
	"strconv"
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
	case "hook":
		return r.runHook(args[1:])
	case "provider":
		return r.runProvider(args[1:])
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
	if err := fs.Parse(args); err != nil {
		return err
	}

	path := r.configFile()
	if config.Exists(path) && !*force {
		return fmt.Errorf("config already exists at %s; use --force to overwrite", path)
	}
	cfg := config.DefaultConfig(path)
	if err := config.Save(path, cfg); err != nil {
		return err
	}
	fmt.Fprintf(r.stdout, "created config: %s\n", path)
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
	if err := fs.Parse(args); err != nil {
		return err
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

func (r runner) runHook(args []string) error {
	if len(args) == 0 {
		return errors.New("hook command requires a subcommand: run, test, install")
	}
	switch args[0] {
	case "run":
		return r.runHookRun(args[1:])
	case "test":
		return r.runHookTest(args[1:])
	case "install":
		return r.runHookInstall(args[1:])
	default:
		return fmt.Errorf("unknown hook subcommand %q", args[0])
	}
}

func (r runner) runHookRun(args []string) error {
	fs := flag.NewFlagSet("hook run", flag.ContinueOnError)
	fs.SetOutput(r.stderr)
	target := fs.String("target", "codex", "hook target")
	eventName := fs.String("event", "user_prompt", "hook event")
	prompt := fs.String("prompt", "", "prompt text")
	workspace := fs.String("workspace", "", "workspace path")
	jsonOut := fs.Bool("json", false, "write JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}

	event := facade.HookEvent{
		Target:    *target,
		Event:     *eventName,
		Prompt:    *prompt,
		Workspace: *workspace,
	}
	if strings.TrimSpace(event.Prompt) == "" {
		bytes, err := io.ReadAll(r.stdin)
		if err != nil {
			return err
		}
		if len(strings.TrimSpace(string(bytes))) > 0 {
			if err := json.Unmarshal(bytes, &event); err != nil {
				return fmt.Errorf("decode hook event JSON: %w", err)
			}
			if event.Target == "" {
				event.Target = *target
			}
			if event.Event == "" {
				event.Event = *eventName
			}
		}
	}
	return r.executeHook(event, *jsonOut)
}

func (r runner) runHookTest(args []string) error {
	fs := flag.NewFlagSet("hook test", flag.ContinueOnError)
	fs.SetOutput(r.stderr)
	target := fs.String("target", "codex", "hook target")
	eventName := fs.String("event", "user_prompt", "hook event")
	prompt := fs.String("prompt", "what did we decide?", "prompt text")
	workspace := fs.String("workspace", "", "workspace path")
	jsonOut := fs.Bool("json", false, "write JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	return r.executeHook(facade.HookEvent{
		Target:    *target,
		Event:     *eventName,
		Prompt:    *prompt,
		Workspace: *workspace,
	}, *jsonOut)
}

func (r runner) runHookInstall(args []string) error {
	fs := flag.NewFlagSet("hook install", flag.ContinueOnError)
	fs.SetOutput(r.stderr)
	target := fs.String("target", "codex", "hook target")
	if err := fs.Parse(args); err != nil {
		return err
	}
	fmt.Fprintf(r.stdout, "hook target: %s\n", *target)
	fmt.Fprintf(r.stdout, "command: paxm --config %s hook run --target %s --event user_prompt\n", r.configFile(), *target)
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

func (r runner) runProvider(args []string) error {
	if len(args) == 0 {
		return errors.New("provider command requires a subcommand: list, enable, disable")
	}
	switch args[0] {
	case "list":
		return r.runProviderList(args[1:])
	case "enable":
		return r.runProviderEnable(args[1:])
	case "disable":
		return r.runProviderDisable(args[1:])
	default:
		return fmt.Errorf("unknown provider subcommand %q", args[0])
	}
}

func (r runner) runProviderList(args []string) error {
	fs := flag.NewFlagSet("provider list", flag.ContinueOnError)
	fs.SetOutput(r.stderr)
	jsonOut := fs.Bool("json", false, "write JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	cfg, err := config.Load(r.configFile())
	if err != nil {
		return err
	}
	if *jsonOut {
		return writeJSON(r.stdout, cfg.Providers)
	}

	var names []string
	for name := range cfg.Providers {
		names = append(names, name)
	}
	sort.Strings(names)
	fmt.Fprintln(r.stdout, "NAME\tTYPE\tENABLED\tREAD\tWRITE\tREQUIRED")
	for _, name := range names {
		provider := cfg.Providers[name]
		fmt.Fprintf(r.stdout, "%s\t%s\t%t\t%t\t%t\t%t\n", name, provider.Type, provider.Enabled, provider.Read, provider.Write, provider.Required)
	}
	return nil
}

func (r runner) runProviderEnable(args []string) error {
	name, flagArgs, err := splitSingleProviderName(args)
	if err != nil {
		return err
	}
	fs := flag.NewFlagSet("provider enable", flag.ContinueOnError)
	fs.SetOutput(r.stderr)
	var read optionalBool
	var write optionalBool
	var required optionalBool
	fs.Var(&read, "read", "set read capability")
	fs.Var(&write, "write", "set write capability")
	fs.Var(&required, "required", "set required status")
	if err := fs.Parse(flagArgs); err != nil {
		return err
	}
	if name == "" {
		return errors.New("provider enable requires a provider name")
	}

	cfg, err := config.Load(r.configFile())
	if err != nil {
		return err
	}
	provider, ok := cfg.Providers[name]
	if !ok {
		return fmt.Errorf("unknown provider %q", name)
	}
	provider.Enabled = true
	if read.set {
		provider.Read = read.value
	}
	if write.set {
		provider.Write = write.value
	}
	if !read.set && !write.set && !provider.Read && !provider.Write {
		provider.Read = true
		provider.Write = true
	}
	if required.set {
		provider.Required = required.value
	}
	cfg.Providers[name] = provider
	if err := config.Save(r.configFile(), cfg); err != nil {
		return err
	}
	fmt.Fprintf(r.stdout, "enabled provider: %s\n", name)
	return nil
}

func (r runner) runProviderDisable(args []string) error {
	fs := flag.NewFlagSet("provider disable", flag.ContinueOnError)
	fs.SetOutput(r.stderr)
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		return errors.New("provider disable requires a provider name")
	}

	cfg, err := config.Load(r.configFile())
	if err != nil {
		return err
	}
	name := fs.Arg(0)
	provider, ok := cfg.Providers[name]
	if !ok {
		return fmt.Errorf("unknown provider %q", name)
	}
	provider.Enabled = false
	cfg.Providers[name] = provider
	if err := config.Save(r.configFile(), cfg); err != nil {
		return err
	}
	fmt.Fprintf(r.stdout, "disabled provider: %s\n", name)
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
	fmt.Fprintln(r.stdout, "  paxm [--config PATH] hook run --target codex --event user_prompt")
	fmt.Fprintln(r.stdout, "  paxm [--config PATH] provider list")
	fmt.Fprintln(r.stdout, "  paxm [--config PATH] config doctor")
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

func splitSingleProviderName(args []string) (string, []string, error) {
	var name string
	var flagArgs []string
	for _, arg := range args {
		if strings.HasPrefix(arg, "-") {
			flagArgs = append(flagArgs, arg)
			continue
		}
		if name != "" {
			return "", nil, errors.New("provider enable accepts exactly one provider name")
		}
		name = arg
	}
	return name, flagArgs, nil
}

type optionalBool struct {
	value bool
	set   bool
}

func (b *optionalBool) Set(value string) error {
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return err
	}
	b.value = parsed
	b.set = true
	return nil
}

func (b *optionalBool) String() string {
	return strconv.FormatBool(b.value)
}

func (b *optionalBool) IsBoolFlag() bool {
	return true
}
