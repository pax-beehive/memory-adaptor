package main

import (
	"os"

	"github.com/pax-beehive/memory-adaptor/internal/cli"
)

func main() {
	os.Exit(cli.Main(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}
