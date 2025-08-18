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

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"

	"github.com/kcp-dev/kcp/cmd/tmc-tui/pkg/config"
)

// TableWidget is a reusable table component with TMC-specific styling and features.
type TableWidget struct {
	config *config.Config
	table  *tview.Table
	title  string
}

// NewTableWidget creates a new table widget with TMC styling.
func NewTableWidget(cfg *config.Config, title string) *TableWidget {
	tw := &TableWidget{
		config: cfg,
		title:  title,
	}

	tw.initializeTable()
	return tw
}

// initializeTable sets up the table with proper styling and configuration.
func (tw *TableWidget) initializeTable() {
	tw.table = tview.NewTable().
		SetBorders(false).
		SetSelectable(true, false).
		SetSeparator(' ')

	// Set title if provided
	if tw.title != "" {
		tw.table.SetTitle(fmt.Sprintf(" %s ", tw.title))
		tw.table.SetTitleAlign(tview.AlignLeft)
	}

	// Apply theme-based styling
	if tw.config.Theme.ShowBorders {
		tw.table.SetBorder(true)
	}

	// Set colors based on theme
	switch tw.config.Theme.ColorScheme {
	case "light":
		tw.table.SetBackgroundColor(tcell.ColorWhite)
	case "dark":
		tw.table.SetBackgroundColor(tcell.ColorBlack)
	default:
		tw.table.SetBackgroundColor(tcell.ColorBlack)
	}

	// Configure selection highlighting
	tw.table.SetSelectedStyle(tcell.Style{}.
		Background(tcell.ColorDarkBlue).
		Foreground(tcell.ColorWhite).
		Attributes(tcell.AttrBold))
}

// SetHeaders sets the column headers for the table.
func (tw *TableWidget) SetHeaders(headers []string) {
	for col, header := range headers {
		cell := tview.NewTableCell(header).
			SetTextColor(tcell.ColorYellow).
			SetAttributes(tcell.AttrBold).
			SetSelectable(false).
			SetAlign(tview.AlignCenter)

		tw.table.SetCell(0, col, cell)
	}
}

// AddRow adds a new row to the table with the provided data.
// Returns the row index of the added row.
func (tw *TableWidget) AddRow(data []string) int {
	row := tw.table.GetRowCount()

	for col, value := range data {
		cell := tview.NewTableCell(value).
			SetTextColor(tcell.ColorWhite).
			SetAlign(tview.AlignLeft)

		tw.table.SetCell(row, col, cell)
	}

	return row
}

// SetCellColor sets the color of a specific cell.
func (tw *TableWidget) SetCellColor(row, col int, color tcell.Color) {
	cell := tw.table.GetCell(row, col)
	if cell != nil {
		cell.SetTextColor(color)
	}
}

// SetRowColor sets the color for all cells in a row.
func (tw *TableWidget) SetRowColor(row int, color tcell.Color) {
	cols := tw.table.GetColumnCount()
	for col := 0; col < cols; col++ {
		tw.SetCellColor(row, col, color)
	}
}

// HighlightHealthStatus colors a cell based on health status.
func (tw *TableWidget) HighlightHealthStatus(row, col int, healthy bool) {
	cell := tw.table.GetCell(row, col)
	if cell == nil {
		return
	}

	if healthy {
		cell.SetTextColor(tcell.ColorGreen)
	} else {
		cell.SetTextColor(tcell.ColorRed)
	}
}

// Clear removes all rows except the header row.
func (tw *TableWidget) Clear() {
	rowCount := tw.table.GetRowCount()
	if rowCount > 1 {
		// Keep header row (row 0), clear the rest
		for row := rowCount - 1; row > 0; row-- {
			tw.table.RemoveRow(row)
		}
	}
}

// SetColumnWidths sets the minimum width for each column.
// Note: SetColumnWidth is not available in this tview version
func (tw *TableWidget) SetColumnWidths(widths []int) {
	// TODO: Implement column width setting when tview supports it
	// For now, this is a no-op
}

// GetRowCount returns the total number of rows including headers.
func (tw *TableWidget) GetRowCount() int {
	return tw.table.GetRowCount()
}

// GetView returns the tview primitive for this table widget.
func (tw *TableWidget) GetView() tview.Primitive {
	return tw.table
}

// Focus sets focus to this table widget.
func (tw *TableWidget) Focus() {
	// Set the table as focused and select first data row if available
	if tw.table.GetRowCount() > 1 {
		tw.table.Select(1, 0) // Skip header row
	}
}

// SetSelectionChangedFunc sets a callback for when the selected row changes.
func (tw *TableWidget) SetSelectionChangedFunc(handler func(row, column int)) {
	tw.table.SetSelectionChangedFunc(handler)
}

// GetSelectedRow returns the currently selected row index.
func (tw *TableWidget) GetSelectedRow() int {
	row, _ := tw.table.GetSelection()
	return row
}