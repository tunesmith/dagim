// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"encoding/json"
	"io"

	"github.com/tunesmith/dagim/internal/diagnostic"
)

type diagnosticOutput = diagnostic.Diagnostic

type jsonEnvelope struct {
	SchemaVersion int                `json:"schema_version"`
	OK            bool               `json:"ok"`
	Result        any                `json:"result"`
	Diagnostics   []diagnosticOutput `json:"diagnostics"`
}

type versionOutput struct {
	Version string `json:"version"`
}

type reportedError struct{ cause error }

func (e reportedError) Error() string { return e.cause.Error() }
func (e reportedError) Unwrap() error { return e.cause }

func writeJSONFailure(w io.Writer, err error) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	if encodeErr := encoder.Encode(jsonEnvelope{
		SchemaVersion: outputSchemaVersion,
		OK:            false,
		Result:        nil,
		Diagnostics:   []diagnosticOutput{diagnostic.FromError(err)},
	}); encodeErr != nil {
		return encodeErr
	}
	return reportedError{cause: err}
}

func diagnosticError(code, message, element string) error {
	return diagnostic.NewError(code, message, element)
}
