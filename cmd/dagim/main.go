// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/tunesmith/dagim/internal/dagimfile"
	"github.com/tunesmith/dagim/internal/graph"
	"github.com/tunesmith/dagim/internal/ui"
)

var version = "dev"

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string) error {
	return runWithIO(args, os.Stdout, os.Stderr)
}

func runWithIO(args []string, stdout, stderr io.Writer) error {
	if len(args) > 0 {
		switch args[0] {
		case "check":
			return runCheckCommand(args[1:], stdout, stderr)
		case "ready":
			return runReadyCommand(args[1:], stdout, stderr)
		case "list":
			return runListCommand(args[1:], stdout, stderr)
		case "show":
			return runShowCommand(args[1:], stdout, stderr)
		case "complete":
			return runCompleteCommand(args[1:], stdout, stderr)
		case "reopen":
			return runReopenCommand(args[1:], stdout, stderr)
		case "help":
			return runHelpCommand(args[1:], stdout)
		}
	}
	return runLegacy(args, stdout, stderr)
}

func runLegacy(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("dagim", flag.ContinueOnError)
	fs.SetOutput(stderr)
	check := fs.Bool("check", false, "parse and validate without opening TUI")
	jsonOutput := fs.Bool("json", false, "output JSON (with --check)")
	showVersion := fs.Bool("version", false, "print version")
	fs.Usage = func() {
		writeTopLevelUsage(fs.Output())
	}
	if err := fs.Parse(flagsFirst(args, map[string]bool{})); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if *showVersion {
		fmt.Fprintln(stdout, versionLine())
		return nil
	}
	if *jsonOutput && !*check {
		return fmt.Errorf("--json requires --check or a subcommand")
	}
	if fs.NArg() != 1 {
		fs.Usage()
		return fmt.Errorf("expected exactly one file")
	}
	path := fs.Arg(0)
	if *check {
		return runCheckTo(path, *jsonOutput, stdout)
	}
	g, err := dagimfile.LoadOrEmpty(path)
	if err != nil {
		return err
	}
	return ui.Run(path, g)
}

func runCheck(path string) error {
	return runCheckTo(path, false, os.Stdout)
}

func runCheckTo(path string, jsonOutput bool, stdout io.Writer) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	result, err := dagimfile.Check(string(data))
	if err != nil {
		return err
	}
	if jsonOutput {
		return writeCheckJSON(stdout, result)
	}
	fmt.Fprintln(stdout, "OK")
	fmt.Fprintf(stdout, "nodes: %d\n", result.Stats.Nodes)
	fmt.Fprintf(stdout, "edges: %d\n", result.Stats.Edges)
	fmt.Fprintf(stdout, "complete: %d\n", result.Stats.Complete)
	fmt.Fprintf(stdout, "ready: %d\n", result.Stats.Ready)
	fmt.Fprintf(stdout, "roots: %d\n", result.Stats.Roots)
	fmt.Fprintf(stdout, "leaves: %d\n", result.Stats.Leaves)
	if result.IsCanonical {
		fmt.Fprintln(stdout, "canonical: yes")
	} else {
		fmt.Fprintln(stdout, "canonical: no")
	}
	fmt.Fprintf(stdout, "transitive_edges: %d\n", len(result.TransitiveEdges))
	for _, edge := range result.TransitiveEdges {
		fmt.Fprintf(stdout, "transitive: %s -> %s via %s\n", edge.Parent, edge.Child, formatPath(edge.Path))
	}
	return nil
}

func writeTopLevelUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage:")
	fmt.Fprintln(w, "  dagim FILE")
	fmt.Fprintln(w, "  dagim check [--json] FILE")
	fmt.Fprintln(w, "  dagim ready [--json] FILE")
	fmt.Fprintln(w, "  dagim list [--state STATE] [--json] FILE")
	fmt.Fprintln(w, "  dagim show [--json] FILE NODE")
	fmt.Fprintln(w, "  dagim complete [--json] FILE NODE")
	fmt.Fprintln(w, "  dagim reopen [--dry-run] [--json] FILE NODE")
	fmt.Fprintln(w, "  dagim --check [--json] FILE")
	fmt.Fprintln(w, "  dagim --version")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Run 'dagim help COMMAND' for command details.")
}

func versionLine() string {
	return "dagim " + version
}

func formatPath(path []graph.NodeID) string {
	parts := make([]string, 0, len(path))
	for _, id := range path {
		parts = append(parts, string(id))
	}
	return strings.Join(parts, " -> ")
}
