package main

import (
	"fmt"
	"os"

	"github.com/ogzhncnmr/doupass/internal/cli"
)

var version = "dev"

func main() {
	cli.Version = version
	if err := cli.Execute(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintln(os.Stderr, "doupass:", err)
		os.Exit(1)
	}
}
