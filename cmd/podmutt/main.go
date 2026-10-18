package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"podmutt/internal/app"
	"podmutt/internal/control"
	"podmutt/internal/macmedia"
)

var version = "dev"

func main() {
	app.SetVersion(version)
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr, func() error {
		return macmedia.Run(func() error { return app.Run(os.Stdin, os.Stdout) })
	}, app.Command))
}

func run(args []string, stdout, stderr io.Writer, startUI func() error, command func(string, io.Writer) error) int {
	flags := flag.NewFlagSet("podmutt", flag.ContinueOnError)
	flags.SetOutput(stderr)
	showVersion := flags.Bool("version", false, "print version and exit")
	flags.Usage = func() {
		fmt.Fprintln(stderr, "Usage: podmutt [--version | --help | play | pause | stop | next | status]")
		flags.PrintDefaults()
	}
	if err := flags.Parse(args); err != nil {
		if err == flag.ErrHelp {
			return 0
		}
		return 2
	}
	if flags.NArg() != 0 {
		if flags.NArg() != 1 || *showVersion || !control.Valid(flags.Arg(0)) {
			fmt.Fprintln(stderr, "podmutt: unexpected arguments")
			return 2
		}
		if err := command(flags.Arg(0), stdout); err != nil {
			fmt.Fprintln(stderr, "podmutt:", err)
			if errors.Is(err, control.ErrNotRunning) {
				return 3
			}
			return 1
		}
		return 0
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
