// SPDX-License-Identifier: GPL-3.0-or-later

package ui

import (
	"io"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

func TestTUIColorsAdaptToTerminalBackground(t *testing.T) {
	light := lipgloss.NewRenderer(io.Discard)
	light.SetColorProfile(termenv.ANSI256)
	light.SetHasDarkBackground(false)
	dark := lipgloss.NewRenderer(io.Discard)
	dark.SetColorProfile(termenv.ANSI256)
	dark.SetHasDarkBackground(true)

	for _, role := range []struct {
		name  string
		style lipgloss.Style
		light string
		dark  string
	}{
		{"muted", mutedStyle, "241", "247"},
		{"error", errorStyle, "124", "203"},
		{"selection", selectStyle, "94", "178"},
		{"completed", completeStyle, "242", "248"},
		{"ready", readyStyle, "22", "42"},
		{"commands", commandStyle, "240", "248"},
	} {
		t.Run(role.name, func(t *testing.T) {
			for _, theme := range []struct {
				name     string
				renderer *lipgloss.Renderer
				color    string
			}{
				{"light", light, role.light},
				{"dark", dark, role.dark},
			} {
				got := role.style.Renderer(theme.renderer).Render("text")
				color := "38;5;" + theme.color
				if !strings.Contains(got, color+"m") && !strings.Contains(got, color+";") {
					t.Errorf("%s terminal rendered %q, want foreground %s", theme.name, got, theme.color)
				}
			}
		})
	}

	for _, renderer := range []*lipgloss.Renderer{light, dark} {
		if got := nodeStyle.Renderer(renderer).Render("focused task"); got != "\x1b[1mfocused task\x1b[0m" {
			t.Errorf("focused task rendered %q, want bold with terminal foreground", got)
		}
	}
}
