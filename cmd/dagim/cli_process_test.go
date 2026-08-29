// SPDX-License-Identifier: GPL-3.0-or-later

package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"strings"
	"testing"
)

const cliHelperEnvironment = "DAGIM_TEST_CLI_HELPER=1"

func TestCLIProcessSuccessUsesStdoutOnly(t *testing.T) {
	path := writeTestGraph(t)
	result := runCLIProcess(t, "ready", path, "--json")
	if result.exitCode != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", result.exitCode, result.stderr)
	}
	if result.stderr != "" {
		t.Fatalf("stderr = %q, want empty", result.stderr)
	}
	envelope, object := decodeSuccessEnvelope(t, []byte(result.stdout))
	assertEnvelope(t, envelope, true)
	assertExactJSONKeys(t, object, "nodes")
}

func TestCLIProcessFailureUsesStderrAndNonzeroExit(t *testing.T) {
	path := writeTestGraph(t)
	result := runCLIProcess(t, "complete", path, "serve-dinner", "--json")
	if result.exitCode != 1 {
		t.Fatalf("exit code = %d, want 1", result.exitCode)
	}
	if result.stderr != "" {
		t.Fatalf("stderr = %q, want empty", result.stderr)
	}
	envelope := decodeJSONObject(t, []byte(result.stdout))
	assertEnvelope(t, envelope, false)
	diagnostics := decodeRawArray(t, envelope["diagnostics"])
	item := decodeRawObject(t, diagnostics[0])
	var code string
	if err := json.Unmarshal(item["code"], &code); err != nil || code != "blocked" {
		t.Fatalf("code = %q, err %v", code, err)
	}
}

func TestCLIProcessJSONUsageFailureUsesEnvelope(t *testing.T) {
	result := runCLIProcess(t, "show", "--json")
	if result.exitCode != 1 || result.stderr != "" {
		t.Fatalf("result = %#v", result)
	}
	envelope := decodeJSONObject(t, []byte(result.stdout))
	assertEnvelope(t, envelope, false)
	diagnostics := decodeRawArray(t, envelope["diagnostics"])
	item := decodeRawObject(t, diagnostics[0])
	var code string
	if err := json.Unmarshal(item["code"], &code); err != nil || code != "usage" {
		t.Fatalf("code = %q, err %v", code, err)
	}
}

func TestCLIProcessUsageFailurePrintsHelpToStderr(t *testing.T) {
	result := runCLIProcess(t, "show")
	if result.exitCode != 1 {
		t.Fatalf("exit code = %d, want 1", result.exitCode)
	}
	if result.stdout != "" {
		t.Fatalf("stdout = %q, want empty", result.stdout)
	}
	if !strings.Contains(result.stderr, "Usage: dagim show") || !strings.Contains(result.stderr, "show expects a file and node ID") {
		t.Fatalf("unexpected stderr: %s", result.stderr)
	}
}

// TestCLIHelperProcess invokes the real main entrypoint in a child copy of the
// test binary so os.Exit behavior and process streams can be asserted.
func TestCLIHelperProcess(t *testing.T) {
	if os.Getenv("DAGIM_TEST_CLI_HELPER") != "1" {
		return
	}
	separator := -1
	for i, arg := range os.Args {
		if arg == "--" {
			separator = i
			break
		}
	}
	if separator < 0 {
		os.Exit(2)
	}
	os.Args = append([]string{"dagim"}, os.Args[separator+1:]...)
	main()
	os.Exit(0)
}

type cliProcessResult struct {
	exitCode int
	stdout   string
	stderr   string
}

func runCLIProcess(t *testing.T, args ...string) cliProcessResult {
	t.Helper()
	commandArgs := append([]string{"-test.run=^TestCLIHelperProcess$", "--"}, args...)
	cmd := exec.Command(os.Args[0], commandArgs...)
	cmd.Env = append(os.Environ(), cliHelperEnvironment)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	exitCode := 0
	if err != nil {
		var exitError *exec.ExitError
		if !errors.As(err, &exitError) {
			t.Fatalf("run CLI process: %v", err)
		}
		exitCode = exitError.ExitCode()
	}
	return cliProcessResult{exitCode: exitCode, stdout: stdout.String(), stderr: stderr.String()}
}
