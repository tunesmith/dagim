// SPDX-License-Identifier: GPL-3.0-or-later

package main

import "testing"

func TestVersionLineUsesInjectedVersion(t *testing.T) {
	original := version
	t.Cleanup(func() {
		version = original
	})

	version = "v1.2.3"

	if got, want := versionLine(), "dagim v1.2.3"; got != want {
		t.Fatalf("versionLine() = %q, want %q", got, want)
	}
}
