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
	"fmt"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/kcp-dev/kcp/cmd/tmc-tui/pkg/config"
)

// Header represents the top header bar of the TUI.
type Header struct {
	config      *config.Config
	view        *tview.Flex
	titleText   *tview.TextView
	currentView string
}

// NewHeader creates a new header component.
func NewHeader(cfg *config.Config) *Header {
	h := &Header{
		config: cfg,
	}

	h.initializeHeader()
	return h
}

// initializeHeader sets up the header layout and styling.
func (h *Header) initializeHeader() {
	// Create title text view
	h.titleText = tview.NewTextView().
		SetDynamicColors(true).
		SetRegions(true).
		SetWrap(false)

	// Set header styling based on theme
	if h.config.Theme.ShowBorders {
		h.titleText.SetBorder(true)
	}

	// Apply color scheme
	switch h.config.Theme.ColorScheme {
	case "light":
		h.titleText.SetBackgroundColor(tcell.ColorWhite)
		h.titleText.SetTextColor(tcell.ColorBlack)
	case "dark":
		h.titleText.SetBackgroundColor(tcell.ColorDarkBlue)
		h.titleText.SetTextColor(tcell.ColorWhite)
	default: // auto or dark as default
		h.titleText.SetBackgroundColor(tcell.ColorDarkBlue)
		h.titleText.SetTextColor(tcell.ColorWhite)
	}

	// Create main flex layout
	h.view = tview.NewFlex().
		AddItem(h.titleText, 0, 1, false)

	h.updateHeader()
}

// updateHeader refreshes the header content.
func (h *Header) updateHeader() {
	now := time.Now()
	timeStr := now.Format("15:04:05")

	title := fmt.Sprintf(`[blue::b]TMC Dashboard[-::-] | [yellow]%s[-] | Last Update: [green]%s[-]`,
		h.getViewDisplayName(h.currentView),
		timeStr)

	h.titleText.SetText(title)
}

// getViewDisplayName returns a human-readable view name.
func (h *Header) getViewDisplayName(viewName string) string {
	switch viewName {
	case "dashboard":
		return "Dashboard"
	case "clusters":
		return "Clusters"
	case "syncers":
		return "Syncers"
	case "metrics":
		return "Metrics"
	default:
		return "Unknown"
	}
}

// SetCurrentView updates the header with the current active view.
func (h *Header) SetCurrentView(viewName string) {
	h.currentView = viewName
	h.updateHeader()
}

// GetView returns the tview component for the header.
func (h *Header) GetView() tview.Primitive {
	return h.view
}

// Update refreshes the header display (typically called by ticker).
func (h *Header) Update() {
	h.updateHeader()
}