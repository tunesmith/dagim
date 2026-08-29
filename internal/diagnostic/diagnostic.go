// SPDX-License-Identifier: GPL-3.0-or-later

package diagnostic

import (
	"errors"
	"fmt"
	"os"

	"github.com/tunesmith/dagim/internal/graph"
)

type Severity string

const (
	SeverityError   Severity = "error"
	SeverityWarning Severity = "warning"
)

type Diagnostic struct {
	Code     string   `json:"code"`
	Message  string   `json:"message"`
	Severity Severity `json:"severity"`
	Line     int      `json:"line,omitempty"`
	Element  string   `json:"element,omitempty"`
}

type Error struct {
	Diagnostic Diagnostic
	Cause      error
}

type Provider interface {
	DiagnosticValue() Diagnostic
}

func NewError(code, message, element string) error {
	return Error{Diagnostic: Diagnostic{Code: code, Message: message, Severity: SeverityError, Element: element}}
}

func Wrap(code, message, element string, cause error) error {
	return Error{Diagnostic: Diagnostic{Code: code, Message: message, Severity: SeverityError, Element: element}, Cause: cause}
}

func (e Error) Error() string {
	if e.Diagnostic.Message != "" {
		return e.Diagnostic.Message
	}
	if e.Cause != nil {
		return e.Cause.Error()
	}
	return e.Diagnostic.Code
}

func (e Error) Unwrap() error { return e.Cause }

func FromError(err error) Diagnostic {
	if err == nil {
		return Diagnostic{}
	}
	var coded Error
	if errors.As(err, &coded) {
		diagnostic := coded.Diagnostic
		if diagnostic.Message == "" {
			diagnostic.Message = err.Error()
		}
		if diagnostic.Severity == "" {
			diagnostic.Severity = SeverityError
		}
		return diagnostic
	}
	var provider Provider
	if errors.As(err, &provider) {
		diagnostic := provider.DiagnosticValue()
		if diagnostic.Message == "" {
			diagnostic.Message = err.Error()
		}
		if diagnostic.Severity == "" {
			diagnostic.Severity = SeverityError
		}
		return diagnostic
	}
	var blocked graph.BlockedError
	if errors.As(err, &blocked) {
		return Diagnostic{Code: "blocked", Message: err.Error(), Severity: SeverityError, Element: string(blocked.Node)}
	}
	var cycle graph.CycleError
	if errors.As(err, &cycle) {
		element := ""
		if len(cycle.Path) > 0 {
			element = string(cycle.Path[0])
		}
		return Diagnostic{Code: "cycle", Message: err.Error(), Severity: SeverityError, Element: element}
	}
	for target, code := range map[error]string{
		graph.ErrEmptyNodeID: "empty_node_id", graph.ErrInvalidNodeID: "invalid_node_id", graph.ErrEmptyNodeText: "empty_node_text",
		graph.ErrDuplicateNode: "duplicate_node", graph.ErrUnknownNode: "unknown_node",
		graph.ErrDuplicateEdge: "duplicate_edge", graph.ErrMissingEdge: "missing_edge", graph.ErrSelfEdge: "self_edge",
		graph.ErrCycle: "cycle", graph.ErrBlocked: "blocked",
	} {
		if errors.Is(err, target) {
			return Diagnostic{Code: code, Message: err.Error(), Severity: SeverityError}
		}
	}
	if errors.Is(err, os.ErrNotExist) {
		return Diagnostic{Code: "file_not_found", Message: err.Error(), Severity: SeverityError, Element: pathElement(err)}
	}
	if errors.Is(err, os.ErrExist) {
		return Diagnostic{Code: "file_changed", Message: err.Error(), Severity: SeverityError, Element: pathElement(err)}
	}
	var pathErr *os.PathError
	if errors.As(err, &pathErr) {
		code := "file_read"
		if pathErr.Op != "open" && pathErr.Op != "stat" {
			code = "file_write"
		}
		return Diagnostic{Code: code, Message: err.Error(), Severity: SeverityError, Element: pathErr.Path}
	}
	var linkErr *os.LinkError
	if errors.As(err, &linkErr) {
		return Diagnostic{Code: "file_write", Message: err.Error(), Severity: SeverityError, Element: linkErr.New}
	}
	return Diagnostic{Code: "internal_error", Message: err.Error(), Severity: SeverityError}
}

func pathElement(err error) string {
	var pathErr *os.PathError
	if errors.As(err, &pathErr) {
		return pathErr.Path
	}
	var linkErr *os.LinkError
	if errors.As(err, &linkErr) {
		return linkErr.New
	}
	return ""
}

func Format(d Diagnostic) string {
	location := ""
	if d.Line > 0 {
		location = fmt.Sprintf(" line %d", d.Line)
	}
	if d.Element != "" {
		location += " " + d.Element
	}
	return fmt.Sprintf("%s [%s]%s: %s", d.Severity, d.Code, location, d.Message)
}
