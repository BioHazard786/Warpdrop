package ui

import (
	"fmt"

	"github.com/BioHazard786/Warpdrop/cli/internal/utils"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
)

// FileTableItem represents a file in the table
type FileTableItem struct {
	Index int
	Name  string
	Size  int64
	Type  string
}

// FileTable renders a beautiful file table using lipgloss/table
type FileTable struct {
	items    []FileTableItem
	showType bool
}

// NewFileTable creates a new file table
func NewFileTable(items []FileTableItem) *FileTable {
	return &FileTable{
		items:    items,
		showType: true,
	}
}

// HideType hides the file type column
func (t *FileTable) HideType() *FileTable {
	t.showType = false
	return t
}

// View renders the table as a string
func (t *FileTable) View() string {
	if len(t.items) == 0 {
		return MutedStyle.Render("No files")
	}

	// Define columns
	var headers []string
	if t.showType {
		headers = []string{"#", "Name", "Size", "Type"}
	} else {
		headers = []string{"#", "Name", "Size"}
	}

	// Define rows
	var rows [][]string
	for _, item := range t.items {
		name := utils.TruncateString(item.Name, 50)
		size := utils.FormatSize(item.Size)

		row := []string{fmt.Sprintf("%d", item.Index), name, size}
		if t.showType {
			fileType := utils.TruncateString(item.Type, 20)
			row = append(row, fileType)
		}
		rows = append(rows, row)
	}

	// Create table
	tbl := table.New().
		Border(lipgloss.NormalBorder()).
		BorderStyle(lipgloss.NewStyle().Foreground(Primary)).
		Headers(headers...).
		Rows(rows...).
		StyleFunc(func(row, col int) lipgloss.Style {
			switch {
			case row == table.HeaderRow:
				return TableHeaderStyle
			case row%2 == 0:
				return TableRowStyle
			default:
				return TableRowAltStyle
			}
		})

	return tbl.Render()
}

// Render outputs the table directly to stdout
func (t *FileTable) Render() {
	fmt.Println(t.View())
}

func RenderFileTable(items []FileTableItem) {
	fmt.Println(NewFileTable(items).View())
}

type TransferSummary struct {
	Status    string
	Files     int
	TotalSize string
	Duration  string
	Speed     string
}

func TransferSummaryView(summary TransferSummary) string {
	headers := []string{"Metric", "Value"}
	rows := [][]string{
		{"Status", summary.Status},
		{"Files", fmt.Sprintf("%d", summary.Files)},
		{"Total Size", summary.TotalSize},
		{"Duration", summary.Duration},
		{"Avg Speed", summary.Speed},
	}

	tbl := table.New().
		Border(lipgloss.NormalBorder()).
		BorderStyle(lipgloss.NewStyle().Foreground(Primary)).
		Headers(headers...).
		Rows(rows...).
		StyleFunc(func(row, col int) lipgloss.Style {
			switch {
			case row == table.HeaderRow:
				return TableHeaderStyle
			case row%2 == 0:
				return TableRowStyle
			default:
				return TableRowAltStyle
			}
		})

	return tbl.Render()
}

func RenderTransferSummary(summary TransferSummary) {
	fmt.Println(TransferSummaryView(summary))
}

type RoomInfo struct {
	RoomID   string
	RoomLink string
}

func NewRoomInfo(roomID, roomLink string) *RoomInfo {
	return &RoomInfo{
		RoomID:   roomID,
		RoomLink: roomLink,
	}
}

func (r *RoomInfo) View() string {
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.DoubleBorder()).
		BorderForeground(Success).
		Padding(1, 2)

	content := fmt.Sprintf("%s Room Created!\n\n%s Room ID:    %s\n%s Room Link:  %s",
		IconSuccess,
		IconCopy, BoldStyle.Foreground(Primary).Render(r.RoomID),
		IconWeb, MutedStyle.Render(r.RoomLink),
	)

	return boxStyle.Render(content)
}
