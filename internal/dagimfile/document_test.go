// SPDX-License-Identifier: GPL-3.0-or-later

package dagimfile

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGraphDetectionRemainsIndependentOfFilenameAndHeader(t *testing.T) {
	for _, name := range []string{"no-extension", "looks-like-a-collection.dagimset", "graph.txt"} {
		for _, content := range []string{"", "node a: First\n", "# dagim v1\nnode a: First\n", "# arbitrary comment\nnode a: First\n"} {
			t.Run(name+"/"+content, func(t *testing.T) {
				path := filepath.Join(t.TempDir(), name)
				if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
					t.Fatal(err)
				}
				isCollection, err := PathIsCollection(path)
				if err != nil || isCollection {
					t.Fatalf("type = %v, %v", isCollection, err)
				}
				g, err := LoadOrEmpty(path)
				if err != nil {
					t.Fatal(err)
				}
				if !strings.HasPrefix(Serialize(g), Header+"\n") {
					t.Fatal("graph format changed")
				}
			})
		}
		path := filepath.Join(t.TempDir(), name)
		g, exists, err := LoadOrEmptyState(path)
		if err != nil || exists || len(g.Nodes()) != 0 {
			t.Fatalf("new graph: exists=%v err=%v", exists, err)
		}
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatal("opening empty graph wrote a file")
		}
	}
}

func TestCollectionHeaderDetectionAndGraphWriteProtection(t *testing.T) {
	for _, content := range []string{"# dagim collection v1\n", "\n \n# dagim collection v1\n{}", "# dagim collection v99\n{}"} {
		if !IsCollection(content) {
			t.Fatalf("not detected: %q", content)
		}
		if _, err := Parse(content); err == nil {
			t.Fatalf("collection accepted as graph: %q", content)
		}
	}
}
