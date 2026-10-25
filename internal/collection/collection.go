// SPDX-License-Identifier: GPL-3.0-or-later

// Package collection stores membership and presentation order, never graph data.
package collection

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/tunesmith/dagim/internal/dagimfile"
	"github.com/tunesmith/dagim/internal/diagnostic"
	"github.com/tunesmith/dagim/internal/graph"
	"github.com/tunesmith/dagim/internal/query"
)

const Header = "# dagim collection v1"

type Member struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

type Ref struct {
	Member string       `json:"member"`
	ID     graph.NodeID `json:"id"`
}

func (r Ref) String() string { return r.Member + ":" + string(r.ID) }

func ParseRef(text string) (Ref, error) {
	member, id, ok := strings.Cut(text, ":")
	if !ok || !validName(member) || !graph.ValidID(graph.NodeID(id)) {
		return Ref{}, diagnostic.NewError("usage", "expected MEMBER:NODE, got "+text, text)
	}
	return Ref{Member: member, ID: graph.NodeID(id)}, nil
}

type Document struct {
	Members []Member `json:"members"`
	Order   []Ref    `json:"order"`
	path    string
	raw     []byte
}

func validName(name string) bool {
	if name == "" {
		return false
	}
	for _, r := range name {
		if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '_' || r == '.') {
			return false
		}
	}
	return name != "." && name != ".."
}

func invalid(message string) error { return diagnostic.NewError("invalid_collection", message, "") }

func (d *Document) validate() error {
	names, paths := map[string]bool{}, map[string]bool{}
	for _, m := range d.Members {
		if !validName(m.Name) || strings.TrimSpace(m.Path) == "" {
			return invalid("members require a name using letters, digits, '.', '_' or '-', and a nonempty path")
		}
		if names[m.Name] || paths[filepath.Clean(m.Path)] {
			return invalid("duplicate collection member: " + m.Name)
		}
		names[m.Name], paths[filepath.Clean(m.Path)] = true, true
	}
	refs := map[Ref]bool{}
	for _, r := range d.Order {
		if !names[r.Member] || !graph.ValidID(r.ID) || refs[r] {
			return invalid("invalid or duplicate order reference: " + r.String())
		}
		refs[r] = true
	}
	return nil
}

func Load(path string) (*Document, error) {
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return nil, err
	}
	resolved, err = filepath.Abs(resolved)
	if err != nil {
		return nil, err
	}
	raw, err := os.ReadFile(resolved)
	if err != nil {
		return nil, err
	}
	text := strings.TrimLeft(string(raw), " \t\r\n")
	header, body, _ := strings.Cut(text, "\n")
	if strings.TrimSpace(header) != Header {
		return nil, invalid("expected " + Header + "; unsupported or missing collection header")
	}
	d := &Document{path: resolved, raw: raw}
	decoder := json.NewDecoder(strings.NewReader(body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(d); err != nil {
		return nil, invalid("invalid collection: " + err.Error())
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return nil, invalid("unexpected content after collection")
	}
	if d.Members == nil || d.Order == nil {
		return nil, invalid("collection requires members and order arrays")
	}
	if err := d.validate(); err != nil {
		return nil, err
	}
	return d, nil
}

func (d *Document) serialize() (string, error) {
	if err := d.validate(); err != nil {
		return "", err
	}
	body, err := json.MarshalIndent(d, "", "  ")
	return Header + "\n" + string(body) + "\n", err
}

func Create(path string) error {
	d := &Document{Members: []Member{}, Order: []Ref{}}
	text, err := d.serialize()
	if err != nil {
		return err
	}
	return dagimfile.CreateTextAtomic(path, text)
}

func (d *Document) Save() error {
	text, err := d.serialize()
	if err != nil {
		return err
	}
	if err := dagimfile.SaveTextIfUnchanged(d.path, text, d.raw); err != nil {
		return err
	}
	d.raw = []byte(text)
	return nil
}

// SourcePath is absolute and independent of the caller's working directory.
func (d *Document) SourcePath(m Member) string {
	if filepath.IsAbs(m.Path) {
		return filepath.Clean(m.Path)
	}
	return filepath.Join(filepath.Dir(d.path), m.Path)
}

func (d *Document) FindMember(name string) (Member, error) {
	for _, m := range d.Members {
		if m.Name == name {
			return m, nil
		}
	}
	return Member{}, diagnostic.NewError("unknown_member", "unknown collection member: "+name, name)
}

func (d *Document) Add(path, name string) error {
	abs, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	if name == "" {
		name = strings.TrimSuffix(filepath.Base(abs), filepath.Ext(abs))
	}
	abs, err = filepath.EvalSymlinks(abs)
	if err != nil {
		return err
	}
	if !validName(name) {
		return invalid("invalid member name; use --as with letters, digits, '.', '_' or '-'")
	}
	for _, m := range d.Members {
		if m.Name == name {
			return invalid("duplicate collection member name: " + name)
		}
	}
	g, err := dagimfile.Load(abs)
	if err != nil {
		return err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return err
	}
	for _, m := range d.Members {
		other, err := os.Stat(d.SourcePath(m))
		if err == nil && os.SameFile(info, other) {
			return invalid("file is already a member as " + m.Name)
		}
	}
	rel, err := filepath.Rel(filepath.Dir(d.path), abs)
	if err != nil {
		return err
	}
	d.Members = append(d.Members, Member{Name: name, Path: rel})
	for _, n := range g.Nodes() {
		d.Order = append(d.Order, Ref{Member: name, ID: n.ID})
	}
	return nil
}

func (d *Document) Remove(name string) error {
	if _, err := d.FindMember(name); err != nil {
		return err
	}
	d.Members = slices.DeleteFunc(d.Members, func(m Member) bool { return m.Name == name })
	d.Order = slices.DeleteFunc(d.Order, func(r Ref) bool { return r.Member == name })
	return nil
}

type Source struct {
	Path  string
	Graph *graph.Graph
	raw   []byte
}

type Snapshot struct {
	Document *Document
	Sources  map[string]Source
}

type Item struct {
	Ref
	File    string
	Summary query.NodeSummary
}

// Snapshot fails visibly if any member is unavailable or invalid. A partial
// frontier must not be mistaken for the complete set of available work.
func (d *Document) Snapshot() (*Snapshot, error) {
	s := &Snapshot{Document: d, Sources: map[string]Source{}}
	infos := []os.FileInfo{}
	for _, m := range d.Members {
		path, err := filepath.EvalSymlinks(d.SourcePath(m))
		if err != nil {
			return nil, fmt.Errorf("member %s: %w", m.Name, err)
		}
		info, err := os.Stat(path)
		if err != nil {
			return nil, err
		}
		for _, other := range infos {
			if os.SameFile(info, other) {
				return nil, invalid("multiple members refer to the same file: " + m.Name)
			}
		}
		infos = append(infos, info)
		raw, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		g, err := dagimfile.Parse(string(raw))
		if err != nil {
			detail := diagnostic.FromError(err)
			detail.Message = fmt.Sprintf("member %s (%s): %s", m.Name, path, err)
			return nil, diagnostic.Error{Diagnostic: detail, Cause: err}
		}
		s.Sources[m.Name] = Source{Path: path, Graph: g, raw: raw}
	}
	return s, nil
}

func LoadSnapshot(path string) (*Snapshot, error) {
	d, err := Load(path)
	if err != nil {
		return nil, err
	}
	return d.Snapshot()
}

// ReadPaths provides an unnamed, read-only selection for multi-file queries.
func ReadPaths(paths []string) (*Snapshot, error) {
	wd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	d := &Document{Members: []Member{}, Order: []Ref{}, path: filepath.Join(wd, ".dagim-selection")}
	for _, path := range paths {
		if err := d.Add(path, ""); err != nil {
			return nil, err
		}
	}
	return d.Snapshot()
}

func (s *Snapshot) Items(filter query.Filter) []Item {
	all := map[Ref]Item{}
	refs := append([]Ref{}, s.Document.Order...)
	seen := map[Ref]bool{}
	for _, m := range s.Document.Members {
		source := s.Sources[m.Name]
		for _, node := range source.Graph.Nodes() {
			r := Ref{Member: m.Name, ID: node.ID}
			all[r] = Item{Ref: r, File: source.Path, Summary: query.Summarize(source.Graph, node)}
			refs = append(refs, r)
		}
	}
	items := []Item{}
	for _, r := range refs {
		if seen[r] {
			continue
		}
		seen[r] = true
		if item, ok := all[r]; ok && query.Matches(item.Summary, filter) {
			items = append(items, item)
		}
	}
	return items
}

func (s *Snapshot) Lookup(r Ref) (Source, error) {
	source, ok := s.Sources[r.Member]
	if !ok {
		return Source{}, diagnostic.NewError("unknown_member", "unknown collection member: "+r.Member, r.Member)
	}
	if !source.Graph.HasNode(r.ID) {
		return Source{}, diagnostic.NewError("unknown_node", "unknown node: "+r.String(), r.String())
	}
	return source, nil
}

// Move persists only the collection order. Hidden and completed nodes retain
// their relative positions. Deleted IDs are retained so reopening/restoring a
// node can recover its old position.
func (s *Snapshot) Move(r, anchor Ref, after bool) error {
	if _, err := s.Lookup(r); err != nil {
		return err
	}
	if _, err := s.Lookup(anchor); err != nil {
		return err
	}
	if r == anchor {
		return nil
	}
	order := append([]Ref{}, s.Document.Order...)
	known := map[Ref]bool{}
	for _, ref := range order {
		known[ref] = true
	}
	for _, item := range s.Items(query.FilterAll) {
		if !known[item.Ref] {
			order = append(order, item.Ref)
			known[item.Ref] = true
		}
	}
	order = slices.DeleteFunc(order, func(ref Ref) bool { return ref == r })
	index := slices.Index(order, anchor)
	if after {
		index++
	}
	order = slices.Insert(order, index, r)
	s.Document.Order = order
	return s.Document.Save()
}

func (s *Snapshot) SetCompletion(r Ref, reopen, dryRun bool) (*graph.Graph, *graph.Graph, error) {
	source, err := s.Lookup(r)
	if err != nil {
		return nil, nil, err
	}
	before := source.Graph
	after := before.Clone()
	if reopen {
		_, err = after.MarkIncompleteCascade(r.ID)
	} else {
		err = after.MarkComplete(r.ID)
	}
	if err != nil {
		return nil, nil, err
	}
	if !dryRun && len(query.Compare(before, after).CompletionChanged) > 0 {
		if err := after.Validate(); err != nil {
			return nil, nil, err
		}
		// Also reject changed membership before writing a selected source.
		current, err := os.ReadFile(s.Document.path)
		if err != nil {
			return nil, nil, err
		}
		if string(current) != string(s.Document.raw) {
			return nil, nil, diagnostic.NewError("file_changed", "collection changed on disk", s.Document.path)
		}
		if err := dagimfile.SaveTextIfUnchanged(source.Path, dagimfile.Serialize(after), source.raw); err != nil {
			return nil, nil, err
		}
	}
	return before, after, nil
}
