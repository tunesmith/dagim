// SPDX-License-Identifier: GPL-3.0-or-later

package dagimfile

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"dagim/internal/graph"
)

const Header = "# dagim v1"

type ParseError struct {
	Line int
	Msg  string
	Err  error
}

func (e ParseError) Error() string {
	msg := e.Msg
	if msg == "" && e.Err != nil {
		msg = e.Err.Error()
	}
	if e.Line <= 0 {
		return msg
	}
	return fmt.Sprintf("line %d: %s", e.Line, msg)
}

func (e ParseError) Unwrap() error {
	return e.Err
}

type CheckResult struct {
	Graph           *graph.Graph
	Stats           graph.Stats
	TransitiveEdges []graph.TransitiveEdge
	Canonical       string
	IsCanonical     bool
}

func Parse(input string) (*graph.Graph, error) {
	g := graph.New()
	type parentRef struct {
		child  graph.NodeID
		parent graph.NodeID
		line   int
	}
	var refs []parentRef
	var current graph.NodeID

	scanner := bufio.NewScanner(strings.NewReader(input))
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		raw := scanner.Text()
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		switch {
		case strings.HasPrefix(line, "node "):
			id, text, err := parseNodeLine(line)
			if err != nil {
				return nil, ParseError{Line: lineNo, Err: err}
			}
			if err := g.AddNodeWithID(id, text); err != nil {
				return nil, ParseError{Line: lineNo, Err: err}
			}
			current = id
		case strings.HasPrefix(line, "parent "):
			if current == "" {
				return nil, ParseError{Line: lineNo, Msg: "parent line before first node"}
			}
			parent, err := parseParentLine(line)
			if err != nil {
				return nil, ParseError{Line: lineNo, Err: err}
			}
			refs = append(refs, parentRef{child: current, parent: parent, line: lineNo})
		case line == "complete" || strings.HasPrefix(line, "complete "):
			if current == "" {
				return nil, ParseError{Line: lineNo, Msg: "complete line before first node"}
			}
			if err := parseCompleteLine(line); err != nil {
				return nil, ParseError{Line: lineNo, Err: err}
			}
			if err := g.SetComplete(current, true); err != nil {
				return nil, ParseError{Line: lineNo, Err: err}
			}
		default:
			return nil, ParseError{Line: lineNo, Msg: fmt.Sprintf("malformed line %q", raw)}
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	for _, ref := range refs {
		if err := g.AddEdge(ref.parent, ref.child); err != nil {
			return nil, ParseError{Line: ref.line, Err: err}
		}
	}
	if err := g.Validate(); err != nil {
		return nil, err
	}
	return g, nil
}

func Serialize(g *graph.Graph) string {
	var b strings.Builder
	b.WriteString(Header)
	b.WriteByte('\n')
	nodes := g.Nodes()
	if len(nodes) == 0 {
		return b.String()
	}
	b.WriteByte('\n')
	for i, node := range nodes {
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString("node ")
		b.WriteString(string(node.ID))
		b.WriteString(": ")
		b.WriteString(node.Text)
		b.WriteByte('\n')
		if node.Complete {
			b.WriteString("  complete\n")
		}
		parents, _ := g.ParentsOf(node.ID)
		for _, parentID := range parents {
			parent, _ := g.Node(parentID)
			b.WriteString("  parent ")
			b.WriteString(string(parentID))
			b.WriteString("  # ")
			b.WriteString(Hint(parent.Text))
			b.WriteByte('\n')
		}
	}
	return b.String()
}

const maxHintRunes = 64

func Hint(text string) string {
	text = strings.Join(strings.Fields(strings.TrimSpace(text)), " ")
	runes := []rune(text)
	if len(runes) <= maxHintRunes {
		return text
	}
	cut := maxHintRunes - 3
	for cut > 40 && !unicode.IsSpace(runes[cut]) {
		cut--
	}
	if cut <= 40 {
		cut = maxHintRunes - 3
	}
	return strings.TrimSpace(string(runes[:cut])) + "..."
}

func Check(input string) (*CheckResult, error) {
	g, err := Parse(input)
	if err != nil {
		return nil, err
	}
	canonical := Serialize(g)
	return &CheckResult{
		Graph:           g,
		Stats:           g.Stats(),
		TransitiveEdges: g.TransitiveEdges(),
		Canonical:       canonical,
		IsCanonical:     input == canonical,
	}, nil
}

func Load(path string) (*graph.Graph, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return Parse(string(data))
}

func LoadOrEmpty(path string) (*graph.Graph, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return graph.New(), nil
	}
	if err != nil {
		return nil, err
	}
	return Parse(string(data))
}

func SaveAtomic(path string, g *graph.Graph) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".dagim-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	if _, err := tmp.WriteString(Serialize(g)); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}

func parseNodeLine(line string) (graph.NodeID, string, error) {
	rest := strings.TrimSpace(strings.TrimPrefix(line, "node "))
	colon := strings.IndexByte(rest, ':')
	if colon < 0 {
		return "", "", fmt.Errorf("node line missing ':'")
	}
	idText := strings.TrimSpace(rest[:colon])
	text := strings.TrimSpace(rest[colon+1:])
	if strings.ContainsAny(idText, " \t") {
		return "", "", fmt.Errorf("node id must be a single token")
	}
	id := graph.NodeID(idText)
	if id == "" {
		return "", "", graph.ErrEmptyNodeID
	}
	if !graph.ValidID(id) {
		return "", "", fmt.Errorf("invalid node id %q", id)
	}
	if text == "" {
		return "", "", graph.ErrEmptyNodeText
	}
	return id, text, nil
}

func parseParentLine(line string) (graph.NodeID, error) {
	rest := strings.TrimSpace(strings.TrimPrefix(line, "parent "))
	if rest == "" || strings.HasPrefix(rest, "#") {
		return "", graph.ErrEmptyNodeID
	}
	fields := strings.Fields(rest)
	if len(fields) == 0 {
		return "", graph.ErrEmptyNodeID
	}
	id := graph.NodeID(fields[0])
	tail := strings.TrimSpace(strings.TrimPrefix(rest, string(id)))
	if tail != "" && !strings.HasPrefix(tail, "#") {
		return "", fmt.Errorf("unexpected text after parent id; use # for optional hint")
	}
	if !graph.ValidID(id) {
		return "", fmt.Errorf("invalid parent id %q", id)
	}
	return id, nil
}

func parseCompleteLine(line string) error {
	rest := strings.TrimSpace(strings.TrimPrefix(line, "complete"))
	if rest != "" && !strings.HasPrefix(rest, "#") {
		return fmt.Errorf("unexpected text after complete; use # for optional comment")
	}
	return nil
}
