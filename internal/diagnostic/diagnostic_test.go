// SPDX-License-Identifier: GPL-3.0-or-later

package diagnostic

import (
	"errors"
	"testing"

	"github.com/tunesmith/dagim/internal/graph"
)

func TestCodedAndDomainErrorsProduceStableDiagnostics(t *testing.T) {
	wrapped := Wrap("usage", "bad arguments", "show", errors.New("cause"))
	got := FromError(wrapped)
	if got.Code != "usage" || got.Message != "bad arguments" || got.Element != "show" || got.Severity != SeverityError {
		t.Fatalf("coded diagnostic = %#v", got)
	}

	got = FromError(graph.BlockedError{Node: "child", Parents: []graph.NodeID{"parent"}})
	if got.Code != "blocked" || got.Element != "child" {
		t.Fatalf("blocked diagnostic = %#v", got)
	}

	got = FromError(graph.CycleError{Path: []graph.NodeID{"a", "b", "a"}})
	if got.Code != "cycle" || got.Element != "a" {
		t.Fatalf("cycle diagnostic = %#v", got)
	}
}

func TestFormatIncludesStableLocation(t *testing.T) {
	got := Format(Diagnostic{Code: "malformed_line", Message: "bad", Severity: SeverityError, Line: 4, Element: "node"})
	if got != "error [malformed_line] line 4 node: bad" {
		t.Fatalf("Format = %q", got)
	}
}
