package main

import (
	"errors"
	"flag"
	"fmt"
	"os"

	"dagim/internal/dagimfile"
	"dagim/internal/ui"
)

const version = "dev"

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string) error {
	fs := flag.NewFlagSet("dagim", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	check := fs.Bool("check", false, "parse and validate without opening TUI")
	showVersion := fs.Bool("version", false, "print version")
	fs.Usage = func() {
		fmt.Fprintln(fs.Output(), "Usage:")
		fmt.Fprintln(fs.Output(), "  dagim FILE")
		fmt.Fprintln(fs.Output(), "  dagim --check FILE")
		fmt.Fprintln(fs.Output(), "  dagim --version")
	}
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if *showVersion {
		fmt.Println("dagim " + version)
		return nil
	}
	if fs.NArg() != 1 {
		fs.Usage()
		return fmt.Errorf("expected exactly one file")
	}
	path := fs.Arg(0)
	if *check {
		return runCheck(path)
	}
	g, err := dagimfile.LoadOrEmpty(path)
	if err != nil {
		return err
	}
	return ui.Run(path, g)
}

func runCheck(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	result, err := dagimfile.Check(string(data))
	if err != nil {
		return err
	}
	fmt.Println("OK")
	fmt.Printf("nodes: %d\n", result.Stats.Nodes)
	fmt.Printf("edges: %d\n", result.Stats.Edges)
	fmt.Printf("roots: %d\n", result.Stats.Roots)
	fmt.Printf("leaves: %d\n", result.Stats.Leaves)
	if result.IsCanonical {
		fmt.Println("canonical: yes")
	} else {
		fmt.Println("canonical: no")
	}
	return nil
}
