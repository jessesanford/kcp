/*
Copyright 2024 The KCP Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package components

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/kcp-dev/kcp/cmd/tmc-tui/pkg/config"
)

// Footer represents the bottom footer bar of the TUI.
type Footer struct {
	config *config.Config
	view   *tview.TextView
}

// NewFooter creates a new footer component.
func NewFooter(cfg *config.Config) *Footer {
	f := &Footer{
		config: cfg,
	}

	f.initializeFooter()
	return f
}

// initializeFooter sets up the footer layout and content.
func (f *Footer) initializeFooter() {
	f.view = tview.NewTextView().
		SetDynamicColors(true).
		SetRegions(true).
		SetWrap(false)

	// Set footer styling based on theme
	if f.config.Theme.ShowBorders {
		f.view.SetBorder(true)
	}

	// Apply color scheme
	switch f.config.Theme.ColorScheme {
	case "light":
		f.view.SetBackgroundColor(tcell.ColorLightGray)
		f.view.SetTextColor(tcell.ColorBlack)
	case "dark":
		f.view.SetBackgroundColor(tcell.ColorDarkSlateGray)
		f.view.SetTextColor(tcell.ColorWhite)
	default: // auto or dark as default
		f.view.SetBackgroundColor(tcell.ColorDarkSlateGray)
		f.view.SetTextColor(tcell.ColorWhite)
	}

	f.updateFooter()
}

// updateFooter refreshes the footer content with keyboard shortcuts.
func (f *Footer) updateFooter() {
	var content string

	if f.config.Features.EnableKeyboardShortcuts {
		shortcuts := []string{
			"[yellow]1-4[-] Views",
			"[yellow]Tab[-] Next",
			"[yellow]r[-] Refresh",
			"[yellow]q[-] Quit",
		}

		if f.config.Features.EnableKeyboardShortcuts {
			shortcuts = append(shortcuts, "[yellow]?[-] Help")
		}

		// Join shortcuts with separators
		content = "[blue::b]Key Bindings:[-::-] "
		for i, shortcut := range shortcuts {
			if i > 0 {
				content += " | "
			}
			content += shortcut
		}
	} else {
		content = "[blue::b]TMC TUI Dashboard[-::-] - Press 'q' to quit"
	}

	f.view.SetText(" " + content)
}

// GetView returns the tview component for the footer.
func (f *Footer) GetView() tview.Primitive {
	return f.view
}

// Update refreshes the footer display.
func (f *Footer) Update() {
	f.updateFooter()
}