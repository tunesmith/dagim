// SPDX-License-Identifier: GPL-3.0-or-later

package collection

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/tunesmith/dagim/internal/dagimfile"
	"github.com/tunesmith/dagim/internal/diagnostic"
	"github.com/tunesmith/dagim/internal/graph"
	"github.com/tunesmith/dagim/internal/query"
)

const testGraph = "# dagim v1\n\nnode d17: D17: First\n\nnode d18: D18: Next\n  parent d17\n"

func fixture(t *testing.T) (string, string, string) {
	t.Helper()
	dir := t.TempDir()
	a, b, path := filepath.Join(dir, "house.anything"), filepath.Join(dir, "work"), filepath.Join(dir, "daily")
	for _, p := range []string{a, b} {
		if err := os.WriteFile(p, []byte(testGraph), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := Create(path); err != nil {
		t.Fatal(err)
	}
	d, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range []Member{{Name: "house", Path: a}, {Name: "work", Path: b}} {
		if err := d.Add(m.Path, m.Name); err != nil {
			t.Fatal(err)
		}
	}
	if err := d.Save(); err != nil {
		t.Fatal(err)
	}
	return path, a, b
}

func refs(items []Item) []string {
	result := []string{}
	for _, item := range items {
		result = append(result, item.Ref.String())
	}
	return result
}

func TestIndependentCompletionAndPersistentOrder(t *testing.T) {
	path, a, b := fixture(t)
	s, err := LoadSnapshot(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := refs(s.Items(query.FilterReady)); !reflect.DeepEqual(got, []string{"house:d17", "work:d17"}) {
		t.Fatal(got)
	}
	if err := s.Move(Ref{"work", "d17"}, Ref{"house", "d17"}, false); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{a, b} {
		raw, _ := os.ReadFile(p)
		if string(raw) != testGraph {
			t.Fatal("ordering changed source", p)
		}
	}
	s, err = LoadSnapshot(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := refs(s.Items(query.FilterReady)); !reflect.DeepEqual(got, []string{"work:d17", "house:d17"}) {
		t.Fatal(got)
	}
	collectionBefore, _ := os.ReadFile(path)
	if _, _, err := s.SetCompletion(Ref{"house", "d17"}, false, false); err != nil {
		t.Fatal(err)
	}
	rawB, _ := os.ReadFile(b)
	if string(rawB) != testGraph {
		t.Fatal("completion changed other source")
	}
	collectionAfter, _ := os.ReadFile(path)
	if string(collectionBefore) != string(collectionAfter) {
		t.Fatal("completion changed collection")
	}
	s, _ = LoadSnapshot(path)
	if got := refs(s.Items(query.FilterReady)); !reflect.DeepEqual(got, []string{"work:d17", "house:d18"}) {
		t.Fatal(got)
	}
	if _, _, err := s.SetCompletion(Ref{"house", "d18"}, false, false); err != nil {
		t.Fatal(err)
	}
	s, _ = LoadSnapshot(path)
	if _, _, err := s.SetCompletion(Ref{"house", "d17"}, true, true); err != nil {
		t.Fatal(err)
	}
	g, _ := dagimfile.Load(a)
	if g.CompleteCount() != 2 {
		t.Fatal("dry run saved")
	}
	if _, _, err := s.SetCompletion(Ref{"house", "d17"}, true, false); err != nil {
		t.Fatal(err)
	}
	g, _ = dagimfile.Load(a)
	if g.CompleteCount() != 0 {
		t.Fatal("reopen did not cascade")
	}
	s, _ = LoadSnapshot(path)
	if got := refs(s.Items(query.FilterReady)); !reflect.DeepEqual(got, []string{"work:d17", "house:d17"}) {
		t.Fatal("lost order", got)
	}
}

func TestMembershipRemovalAndRelativePaths(t *testing.T) {
	path, a, _ := fixture(t)
	d, _ := Load(path)
	if filepath.IsAbs(d.Members[0].Path) {
		t.Fatal("stored absolute member path")
	}
	// Moving the entire folder preserves member resolution from another cwd.
	moved := filepath.Join(t.TempDir(), "moved")
	if err := os.Rename(filepath.Dir(path), moved); err != nil {
		t.Fatal(err)
	}
	path = filepath.Join(moved, filepath.Base(path))
	s, err := LoadSnapshot(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Items(query.FilterReady)) != 2 {
		t.Fatal("relative member paths failed")
	}
	d = s.Document
	// Membership remains repairable even when a member is missing.
	if err := os.Remove(filepath.Join(moved, "work")); err != nil {
		t.Fatal(err)
	}
	if _, err := d.Snapshot(); err == nil {
		t.Fatal("missing member silently omitted")
	}
	if err := d.Remove("work"); err != nil {
		t.Fatal(err)
	}
	if err := d.Save(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(moved, filepath.Base(a))); err != nil {
		t.Fatal("remove deleted source", err)
	}
	s, err = LoadSnapshot(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range s.Document.Order {
		if r.Member == "work" {
			t.Fatal("removed member order retained")
		}
	}
}

func TestStaleWritesAndBlockedCompletion(t *testing.T) {
	path, a, _ := fixture(t)
	s, _ := LoadSnapshot(path)
	if _, _, err := s.SetCompletion(Ref{"house", "d18"}, false, false); diagnostic.FromError(err).Code != "blocked" {
		t.Fatal(err)
	}
	if err := os.WriteFile(a, []byte(testGraph+"\n# external change\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.SetCompletion(Ref{"house", "d17"}, false, false); diagnostic.FromError(err).Code != "file_changed" {
		t.Fatal(err)
	}
	s, _ = LoadSnapshot(path)
	d, _ := Load(path)
	if err := d.Remove("house"); err != nil {
		t.Fatal(err)
	}
	if err := d.Save(); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.SetCompletion(Ref{"house", "d17"}, false, false); diagnostic.FromError(err).Code != "file_changed" {
		t.Fatal(err)
	}
	if err := s.Move(Ref{"house", "d17"}, Ref{"work", "d17"}, false); diagnostic.FromError(err).Code != "file_changed" {
		t.Fatal(err)
	}
	current, _ := Load(path)
	if len(current.Members) != 1 {
		t.Fatal("stale save overwrote membership")
	}
}

func TestInvalidCollectionsAndGraphProtection(t *testing.T) {
	path, a, _ := fixture(t)
	d, _ := Load(path)
	if err := d.Add(a, "again"); err == nil {
		t.Fatal("duplicate physical member accepted")
	}
	link := filepath.Join(filepath.Dir(a), "link")
	if err := os.Symlink(a, link); err != nil {
		t.Fatal(err)
	}
	if err := d.Add(link, "alias"); err == nil {
		t.Fatal("duplicate symlink accepted")
	}
	if err := d.Add(path, "nested"); err == nil {
		t.Fatal("nested collection accepted")
	}
	for _, text := range []string{
		Header, Header + "\n{}", Header + "\nnull", "# dagim collection v2\n{}",
		Header + "\n" + `{"members":null,"order":[]}`,
		Header + "\n" + `{"members":[],"order":[],"unexpected":true}`,
		Header + "\n" + `{"members":[{"name":"a","path":"a"},{"name":"a","path":"b"}],"order":[]}`,
		Header + "\n" + `{"members":[{"name":"a","path":"a"}],"order":[{"member":"missing","id":"x"}]}`,
		Header + "\n" + `{"members":[{"name":"a","path":"a"}],"order":[{"member":"a","id":"x"},{"member":"a","id":"x"}]}`,
		Header + "\n{\"members\":[],\"order\":[]}\n{}",
	} {
		p := filepath.Join(t.TempDir(), "bad")
		if err := os.WriteFile(p, []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := Load(p); err == nil {
			t.Fatalf("accepted %q", text)
		}
	}
	if _, err := dagimfile.Parse(Header + "\n"); err == nil {
		t.Fatal("graph parser accepted empty collection")
	}
	if err := Create(a); err == nil {
		t.Fatal("create overwrote graph")
	}
	raw, _ := os.ReadFile(a)
	if string(raw) != testGraph {
		t.Fatal("create changed source")
	}
	if _, err := ParseRef("d17"); err == nil {
		t.Fatal("unqualified reference accepted")
	}
	if _, err := ParseRef("house:d17"); err != nil {
		t.Fatal(err)
	}
}

func TestNewTasksAppendAndSymlinkCompletion(t *testing.T) {
	path, a, _ := fixture(t)
	s, _ := LoadSnapshot(path)
	if err := s.Move(Ref{"work", "d17"}, Ref{"house", "d17"}, false); err != nil {
		t.Fatal(err)
	}
	g, _ := dagimfile.Load(a)
	if err := g.AddNodeWithID(graph.NodeID("new"), "New task"); err != nil {
		t.Fatal(err)
	}
	if err := dagimfile.SaveAtomic(a, g); err != nil {
		t.Fatal(err)
	}
	s, _ = LoadSnapshot(path)
	got := refs(s.Items(query.FilterReady))
	if !reflect.DeepEqual(got, []string{"work:d17", "house:d17", "house:new"}) {
		t.Fatal(got)
	}
	// Loading the collection through a symlink must retain its real base path.
	link := filepath.Join(t.TempDir(), "daily-link")
	if err := os.Symlink(path, link); err != nil {
		t.Fatal(err)
	}
	s, err := LoadSnapshot(link)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.SetCompletion(Ref{"house", "new"}, false, false); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Readlink(link); err != nil {
		t.Fatal("save replaced symlink")
	}
}
