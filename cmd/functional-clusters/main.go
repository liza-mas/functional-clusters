package main

import (
	"os"

	"github.com/liza-mas/functional-clusters/internal/cli"
)

func main() {
	os.Exit(cli.Run(os.Args[1:], os.Stdout, os.Stderr))
}
