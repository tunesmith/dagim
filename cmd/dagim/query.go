// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/tunesmith/dagim/internal/dagimfile"
	"github.com/tunesmith/dagim/internal/graph"
	domainquery "github.com/tunesmith/dagim/internal/query"
)

const outputSchemaVersion = 2

type nodeOutput struct {
	ID        string   `json:"id"`
	Text      string   `json:"text"`
	State     string   `json:"state"`
	Complete  bool     `json:"complete"`
	Ready     bool     `json:"ready"`
	BlockedBy []string `json:"blocked_by"`
}

type nodeListOutput struct {
	Nodes []nodeOutput `json:"nodes"`
}

type nodeShowOutput struct {
	Node     nodeOutput   `json:"node"`
	Parents  []nodeOutput `json:"parents"`
	Children []nodeOutput `json:"children"`
}

type statsOutput struct {
	Nodes    int `json:"nodes"`
	Edges    int `json:"edges"`
	Complete int `json:"complete"`
	Ready    int `json:"ready"`
	Roots    int `json:"roots"`
	Leaves   int `json:"leaves"`
}

type transitiveEdgeOutput struct {
	Parent string   `json:"parent"`
	Child  string   `json:"child"`
	Path   []string `json:"path"`
}

type checkOutput struct {
	OK              bool                   `json:"ok"`
	Stats           statsOutput            `json:"stats"`
	Canonical       bool                   `json:"canonical"`
	TransitiveEdges []transitiveEdgeOutput `json:"transitive_edges"`
}

func runCheckCommand(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("dagim check", flag.ContinueOnError)
	fs.SetOutput(stderr)
	jsonOutput := fs.Bool("json", false, "output JSON")
	fs.Usage = func() { writeCheckUsage(fs.Output()) }
	if err := fs.Parse(flagsFirst(args, nil)); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return diagnosticError("usage", err.Error(), "check")
	}
	if fs.NArg() != 1 {
		fs.Usage()
		return diagnosticError("usage", "check expects exactly one file", "check")
	}
	return runCheckTo(fs.Arg(0), *jsonOutput, stdout)
}

func runReadyCommand(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("dagim ready", flag.ContinueOnError)
	fs.SetOutput(stderr)
	jsonOutput := fs.Bool("json", false, "output JSON")
	fs.Usage = func() { writeReadyUsage(fs.Output()) }
	if err := fs.Parse(flagsFirst(args, nil)); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return diagnosticError("usage", err.Error(), "ready")
	}
	if fs.NArg() != 1 {
		fs.Usage()
		return diagnosticError("usage", "ready expects exactly one file", "ready")
	}
	return writeNodeList(fs.Arg(0), "ready", *jsonOutput, stdout)
}

func runListCommand(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("dagim list", flag.ContinueOnError)
	fs.SetOutput(stderr)
	jsonOutput := fs.Bool("json", false, "output JSON")
	state := fs.String("state", "all", "filter by all, ready, blocked, complete, or incomplete")
	fs.Usage = func() { writeListUsage(fs.Output()) }
	if err := fs.Parse(flagsFirst(args, map[string]bool{"state": true})); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return diagnosticError("usage", err.Error(), "list")
	}
	if fs.NArg() != 1 {
		fs.Usage()
		return diagnosticError("usage", "list expects exactly one file", "list")
	}
	filter := domainquery.Filter(*state)
	if !domainquery.ValidFilter(filter) {
		return diagnosticError("unknown_state", fmt.Sprintf("unknown state %q; want all, ready, blocked, complete, or incomplete", *state), *state)
	}
	return writeNodeList(fs.Arg(0), filter, *jsonOutput, stdout)
}

func runShowCommand(args []string, stdout, stderr io.Writer) error {
	fs := flag.NewFlagSet("dagim show", flag.ContinueOnError)
	fs.SetOutput(stderr)
	jsonOutput := fs.Bool("json", false, "output JSON")
	fs.Usage = func() { writeShowUsage(fs.Output()) }
	if err := fs.Parse(flagsFirst(args, nil)); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return diagnosticError("usage", err.Error(), "show")
	}
	if fs.NArg() != 2 {
		fs.Usage()
		return diagnosticError("usage", "show expects a file and node ID", "show")
	}

	g, err := dagimfile.Load(fs.Arg(0))
	if err != nil {
		return err
	}
	id := graph.NodeID(fs.Arg(1))
	if !g.HasNode(id) {
		return diagnosticError("unknown_node", fmt.Sprintf("%s: %s", graph.ErrUnknownNode, id), string(id))
	}
	relations, err := domainquery.RelationsFor(g, id)
	if err != nil {
		return err
	}
	result := nodeShowOutput{
		Node: summarizeNode(relations.Node), Parents: summarizeNodes(relations.Parents), Children: summarizeNodes(relations.Children),
	}
	if *jsonOutput {
		return writeJSON(stdout, result)
	}
	writeNodeShow(stdout, result)
	return nil
}

func runHelpCommand(args []string, stdout io.Writer) error {
	if len(args) == 0 {
		writeTopLevelUsage(stdout)
		return nil
	}
	if len(args) != 1 {
		return diagnosticError("usage", "help expects at most one command", "help")
	}
	switch args[0] {
	case "check":
		writeCheckUsage(stdout)
	case "ready":
		writeReadyUsage(stdout)
	case "list":
		writeListUsage(stdout)
	case "show":
		writeShowUsage(stdout)
	case "complete":
		writeCompleteUsage(stdout)
	case "reopen":
		writeReopenUsage(stdout)
	case "add":
		writeAddUsage(stdout)
	case "edit":
		writeEditUsage(stdout)
	case "link":
		writeLinkUsage(stdout)
	case "unlink":
		writeUnlinkUsage(stdout)
	case "delete":
		writeDeleteUsage(stdout)
	default:
		return diagnosticError("unknown_command", fmt.Sprintf("unknown command %q", args[0]), args[0])
	}
	return nil
}

func writeNodeList(path string, filter domainquery.Filter, jsonOutput bool, stdout io.Writer) error {
	g, err := dagimfile.Load(path)
	if err != nil {
		return err
	}
	nodes := summarizeNodes(domainquery.List(g, filter))
	if jsonOutput {
		return writeJSON(stdout, nodeListOutput{Nodes: nodes})
	}
	for _, node := range nodes {
		fmt.Fprintf(stdout, "%s\t%s\t%s", node.State, node.ID, node.Text)
		if len(node.BlockedBy) > 0 {
			fmt.Fprintf(stdout, "\tblocked by: %s", strings.Join(node.BlockedBy, ", "))
		}
		fmt.Fprintln(stdout)
	}
	return nil
}

func summarizeNodes(nodes []domainquery.NodeSummary) []nodeOutput {
	result := make([]nodeOutput, 0, len(nodes))
	for _, node := range nodes {
		result = append(result, summarizeNode(node))
	}
	return result
}

func summarizeNode(summary domainquery.NodeSummary) nodeOutput {
	blockedBy := make([]string, 0, len(summary.BlockedBy))
	for _, id := range summary.BlockedBy {
		blockedBy = append(blockedBy, string(id))
	}
	return nodeOutput{
		ID:        string(summary.Node.ID),
		Text:      summary.Node.Text,
		State:     string(summary.State),
		Complete:  summary.Node.Complete,
		Ready:     summary.State == domainquery.StateReady,
		BlockedBy: blockedBy,
	}
}

func writeNodeShow(w io.Writer, result nodeShowOutput) {
	node := result.Node
	fmt.Fprintf(w, "id: %s\n", node.ID)
	fmt.Fprintf(w, "text: %s\n", node.Text)
	fmt.Fprintf(w, "state: %s\n", node.State)
	if len(node.BlockedBy) > 0 {
		fmt.Fprintf(w, "blocked_by: %s\n", strings.Join(node.BlockedBy, ", "))
	}
	writeRelations := func(label string, nodes []nodeOutput) {
		fmt.Fprintf(w, "%s:\n", label)
		if len(nodes) == 0 {
			fmt.Fprintln(w, "  (none)")
			return
		}
		for _, related := range nodes {
			fmt.Fprintf(w, "  %s\t%s\t%s\n", related.State, related.ID, related.Text)
		}
	}
	writeRelations("parents", result.Parents)
	writeRelations("children", result.Children)
}

func writeCheckJSON(w io.Writer, result *dagimfile.CheckResult) error {
	edges := make([]transitiveEdgeOutput, 0, len(result.TransitiveEdges))
	for _, edge := range result.TransitiveEdges {
		path := make([]string, 0, len(edge.Path))
		for _, id := range edge.Path {
			path = append(path, string(id))
		}
		edges = append(edges, transitiveEdgeOutput{
			Parent: string(edge.Parent),
			Child:  string(edge.Child),
			Path:   path,
		})
	}
	return writeJSON(w, checkOutput{
		OK:              true,
		Stats:           makeStatsOutput(result.Stats),
		Canonical:       result.IsCanonical,
		TransitiveEdges: edges,
	})
}

func writeJSON(w io.Writer, value any) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(jsonEnvelope{SchemaVersion: outputSchemaVersion, OK: true, Result: value, Diagnostics: []diagnosticOutput{}})
}

// flagsFirst lets the small standard-library flag parser accept flags either
// before or after positional arguments, as users commonly expect from CLIs.
func flagsFirst(args []string, valueFlags map[string]bool) []string {
	flags := make([]string, 0, len(args))
	positionals := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			positionals = append(positionals, args[i+1:]...)
			break
		}
		if !strings.HasPrefix(arg, "-") || arg == "-" {
			positionals = append(positionals, arg)
			continue
		}
		flags = append(flags, arg)
		name := strings.TrimLeft(arg, "-")
		if before, _, found := strings.Cut(name, "="); found {
			name = before
		}
		if valueFlags[name] && !strings.Contains(arg, "=") && i+1 < len(args) {
			i++
			flags = append(flags, args[i])
		}
	}
	return append(flags, positionals...)
}

func writeCheckUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage: dagim check [--json] FILE")
	fmt.Fprintln(w, "Parse and validate a graph without opening the TUI.")
}

func writeReadyUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage: dagim ready [--json] FILE")
	fmt.Fprintln(w, "List incomplete nodes whose parents are complete.")
}

func writeListUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage: dagim list [--state STATE] [--json] FILE")
	fmt.Fprintln(w, "List nodes, optionally filtered by all, ready, blocked, complete, or incomplete.")
}

func writeShowUsage(w io.Writer) {
	fmt.Fprintln(w, "Usage: dagim show [--json] FILE NODE")
	fmt.Fprintln(w, "Show a node with its state, blockers, parents, and children.")
}
