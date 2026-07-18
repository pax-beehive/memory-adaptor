package cli

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"time"

	paxruntime "github.com/pax-beehive/paxm/internal/runtime"
	"github.com/pax-beehive/paxm/internal/telemetry"
)

func (r runner) runConfig(args []string) error {
	if len(args) == 0 {
		return errors.New("config command requires a subcommand: path, show, doctor")
	}
	switch args[0] {
	case "path":
		_, _ = fmt.Fprintln(r.stdout, r.configFile())
		return nil
	case "show":
		_, rt, err := r.loadRuntime()
		if err != nil {
			return err
		}
		defer func() { _ = rt.Close() }()
		return writeJSON(r.stdout, rt.Operator.Config())
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
	rt, err := paxruntime.Load(r.configFile())
	if err != nil {
		return err
	}
	defer func() { _ = rt.Close() }()
	statuses, err := rt.Operator.Health(context.Background())
	if *jsonOut {
		if writeErr := writeJSON(r.stdout, statuses); writeErr != nil {
			return writeErr
		}
		return err
	}
	for _, status := range statuses {
		if status.OK {
			_, _ = fmt.Fprintf(r.stdout, "ok: %s\n", status.Provider)
			continue
		}
		_, _ = fmt.Fprintf(r.stdout, "error: %s: %s\n", status.Provider, status.Error)
	}
	return err
}

func (r runner) runHistory(args []string) error {
	fs := flag.NewFlagSet("history", flag.ContinueOnError)
	fs.SetOutput(r.stderr)
	days := fs.Int("days", 7, "number of days to summarize")
	jsonOut := fs.Bool("json", false, "write JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	_, rt, err := r.loadRuntime()
	if err != nil {
		return err
	}
	defer func() { _ = rt.Close() }()
	summary, err := rt.Operator.History(*days)
	if err != nil {
		return err
	}
	if *jsonOut {
		return writeJSON(r.stdout, summary)
	}
	writeHistorySummary(r.stdout, summary)
	return nil
}

func (r runner) runLogs(args []string) error {
	fs := flag.NewFlagSet("logs", flag.ContinueOnError)
	fs.SetOutput(r.stderr)
	tail := fs.Int("tail", 50, "number of recent events")
	follow := fs.Bool("follow", false, "follow new events")
	jsonOut := fs.Bool("json", false, "write JSONL")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *tail < 0 {
		return errors.New("logs tail must be non-negative")
	}
	_, rt, err := r.loadRuntime()
	if err != nil {
		return err
	}
	defer func() { _ = rt.Close() }()
	emit := func(event telemetry.Event) error {
		if *jsonOut {
			return json.NewEncoder(r.stdout).Encode(event)
		}
		writeLogEvent(r.stdout, event)
		return nil
	}
	if *follow {
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
		defer stop()
		return rt.Operator.FollowEvents(ctx, *tail, 250*time.Millisecond, emit)
	}
	events, err := rt.Operator.TailEvents(*tail)
	if err != nil {
		return err
	}
	for _, event := range events {
		if err := emit(event); err != nil {
			return err
		}
	}
	return nil
}
