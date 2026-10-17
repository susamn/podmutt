package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"podmutt/internal/app"
)

var version = "dev"

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr, func() error {
		return app.Run(os.Stdin, os.Stdout)
	}))
}

func run(args []string, stdout, stderr io.Writer, startUI func() error) int {
	flags := flag.NewFlagSet("podmutt", flag.ContinueOnError)
	flags.SetOutput(stderr)
	showVersion := flags.Bool("version", false, "print version and exit")
	flags.Usage = func() {
		fmt.Fprintln(stderr, "Usage: podmutt [--version | --help]")
		flags.PrintDefaults()
	}
	if err := flags.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}
	if flags.NArg() != 0 {
		fmt.Fprintln(stderr, "podmutt: unexpected arguments")
		return 2
	}
	if *showVersion {
		fmt.Fprintln(stdout, "podmutt", version)
		return 0
	}
	if err := startUI(); err != nil {
		fmt.Fprintln(stderr, "podmutt:", err)
		return 1
	}
	return 0
}
