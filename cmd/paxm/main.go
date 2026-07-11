package main

import (
	"os"

	"github.com/pax-beehive/memory-adaptor/internal/cli"
)

var version = "dev"

func main() {
	os.Exit(cli.MainWithDependencies(os.Args[1:], os.Stdin, os.Stdout, os.Stderr, cli.Dependencies{
		Version: version,
	}))
}
