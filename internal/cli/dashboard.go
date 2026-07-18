package cli

import (
	"context"
	"flag"
	"os/signal"
	"syscall"

	"github.com/pax-beehive/paxm/internal/dashboard"
)

func (r runner) runDashboard(args []string) error {
	fs := flag.NewFlagSet("dashboard", flag.ContinueOnError)
	fs.SetOutput(r.stderr)
	addr := fs.String("addr", "127.0.0.1:7465", "listen address (loopback only)")
	days := fs.Int("days", 7, "metrics window in days")
	if err := fs.Parse(args); err != nil {
		return err
	}
	_, rt, err := r.loadRuntime()
	if err != nil {
		return err
	}
	defer func() { _ = rt.Close() }()
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	return dashboard.New(rt.Operator, *days).Serve(ctx, *addr, r.stdout)
}
