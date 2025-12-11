package ui

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/jedib0t/go-pretty/v6/table"
)

// FileTableItem represents a file in the table
type FileTableItem struct {
	Index int
	Name  string
	Size  int64
	Type  string
}

// FileTable renders a beautiful file table using go-pretty
type FileTable struct {
	items    []FileTableItem
	title    string
	showType bool
}

// NewFileTable creates a new file table
func NewFileTable(items []FileTableItem, title string) *FileTable {
	return &FileTable{
		items:    items,
		title:    title,
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

	tw := table.NewWriter()
	tw.SetStyle(table.StyleColoredBright)

	// Set title if provided
	if t.title != "" {
		tw.SetTitle(t.title)
	}

	// Set header
	if t.showType {
		tw.AppendHeader(table.Row{"#", "Name", "Size", "Type"})
	} else {
		tw.AppendHeader(table.Row{"#", "Name", "Size"})
	}

	// Add rows
	for _, item := range t.items {
		name := truncateString(item.Name, 50)
		size := formatBytes(item.Size)
		fileType := truncateString(item.Type, 20)

		if t.showType {
			tw.AppendRow(table.Row{item.Index, name, size, fileType})
		} else {
			tw.AppendRow(table.Row{item.Index, name, size})
		}
	}

	return tw.Render()
}

// Render outputs the table directly to stdout
func (t *FileTable) Render() {
	if len(t.items) == 0 {
		fmt.Println(MutedStyle.Render("No files"))
		return
	}

	tw := table.NewWriter()
	tw.SetOutputMirror(os.Stdout)
	tw.SetStyle(table.StyleColoredBright)

	// Set title if provided
	if t.title != "" {
		tw.SetTitle(t.title)
	}

	// Set header
	if t.showType {
		tw.AppendHeader(table.Row{"#", "Name", "Size", "Type"})
	} else {
		tw.AppendHeader(table.Row{"#", "Name", "Size"})
	}

	// Add rows
	for _, item := range t.items {
		name := truncateString(item.Name, 50)
		size := formatBytes(item.Size)
		fileType := truncateString(item.Type, 20)

		if t.showType {
			tw.AppendRow(table.Row{item.Index, name, size, fileType})
		} else {
			tw.AppendRow(table.Row{item.Index, name, size})
		}
	}

	tw.Render()
}

// RenderFileTable is a convenience function to quickly render a file table
func RenderFileTable(items []FileTableItem, title string) {
	NewFileTable(items, title).Render()
}

// TransferSummary represents transfer statistics
type TransferSummary struct {
	Status    string
	Files     int
	TotalSize string
	Duration  string
	Speed     string
}

// RenderTransferSummary renders a transfer summary table using go-pretty
func RenderTransferSummary(title string, summary TransferSummary) {
	tw := table.NewWriter()
	tw.SetOutputMirror(os.Stdout)
	tw.SetStyle(table.StyleColoredBright)
	tw.SetTitle(title)

	tw.AppendHeader(table.Row{"Metric", "Value"})
	tw.AppendRows([]table.Row{
		{"Status", summary.Status},
		{"Files", summary.Files},
		{"Total Size", summary.TotalSize},
		{"Duration", summary.Duration},
		{"Avg Speed", summary.Speed},
	})

	tw.Render()
}

// Summary renders a summary box
type Summary struct {
	title  string
	items  []SummaryItem
	status SummaryStatus
}

type SummaryItem struct {
	Label string
	Value string
}

type SummaryStatus int

const (
	SummaryStatusNormal SummaryStatus = iota
	SummaryStatusSuccess
	SummaryStatusError
	SummaryStatusWarning
)

func NewSummary(title string, status SummaryStatus) *Summary {
	return &Summary{
		title:  title,
		status: status,
		items:  make([]SummaryItem, 0),
	}
}

func (s *Summary) AddItem(label, value string) *Summary {
	s.items = append(s.items, SummaryItem{Label: label, Value: value})
	return s
}

func (s *Summary) View() string {
	var borderColor lipgloss.Color
	var titleStyle lipgloss.Style

	switch s.status {
	case SummaryStatusSuccess:
		borderColor = Success
		titleStyle = SuccessStyle
	case SummaryStatusError:
		borderColor = Error
		titleStyle = ErrorStyle
	case SummaryStatusWarning:
		borderColor = Warning
		titleStyle = WarningStyle
	default:
		borderColor = Primary
		titleStyle = TitleStyle
	}

	// Calculate max width
	maxLabelLen := 0
	maxValueLen := 0
	for _, item := range s.items {
		if len(item.Label) > maxLabelLen {
			maxLabelLen = len(item.Label)
		}
		if len(item.Value) > maxValueLen {
			maxValueLen = len(item.Value)
		}
	}

	totalWidth := maxLabelLen + maxValueLen + 8
	if len(s.title) > totalWidth {
		totalWidth = len(s.title) + 4
	}

	var b strings.Builder

	// Top border with title
	topBorder := "╭─ " + titleStyle.Render(s.title) + " " +
		lipgloss.NewStyle().Foreground(borderColor).Render(strings.Repeat("─", totalWidth-len(s.title)-4)+"╮")
	b.WriteString(topBorder + "\n")

	// Items
	labelStyle := lipgloss.NewStyle().Foreground(Muted)
	valueStyle := lipgloss.NewStyle().Bold(true)

	for _, item := range s.items {
		line := fmt.Sprintf("│  %s %s",
			labelStyle.Width(maxLabelLen+1).Render(item.Label+":"),
			valueStyle.Render(item.Value),
		)
		padding := totalWidth + 1 - lipgloss.Width(line)
		if padding < 0 {
			padding = 0
		}
		line += strings.Repeat(" ", padding)
		line += lipgloss.NewStyle().Foreground(borderColor).Render("│")
		b.WriteString(lipgloss.NewStyle().Foreground(borderColor).Render("│") + line[1:] + "\n")
	}

	// Bottom border
	bottomBorder := lipgloss.NewStyle().Foreground(borderColor).Render(
		"╰" + strings.Repeat("─", totalWidth+1) + "╯",
	)
	b.WriteString(bottomBorder)

	return b.String()
}

// RoomInfo displays room connection information
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

	content := fmt.Sprintf("%s Room Created!\n\n%s Room ID (CLI):    %s\n%s Room Link (Web):  %s",
		IconSuccess,
		IconCopy, BoldStyle.Foreground(Primary).Render(r.RoomID),
		IconWeb, MutedStyle.Render(r.RoomLink),
	)

	return boxStyle.Render(content)
}

// ConfirmPrompt is a styled confirmation prompt
type ConfirmPrompt struct {
	message string
}

func NewConfirmPrompt(message string) *ConfirmPrompt {
	return &ConfirmPrompt{message: message}
}

func (p *ConfirmPrompt) View() string {
	return fmt.Sprintf("\n%s %s [Y/n] ",
		WarningStyle.Render("?"),
		p.message,
	)
}
