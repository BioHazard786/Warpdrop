package ui

import (
	"github.com/charmbracelet/lipgloss"
)

// Color palette
var (
	// Primary colors - using WarpDrop accent color
	Primary    = lipgloss.Color("#22d3ee") // WarpDrop Cyan accent
	Secondary  = lipgloss.Color("#7C3AED") // Violet
	Success    = lipgloss.Color("#10B981") // Emerald
	Warning    = lipgloss.Color("#F59E0B") // Amber
	Error      = lipgloss.Color("#EF4444") // Red
	Muted      = lipgloss.Color("#6B7280") // Gray
	Foreground = lipgloss.Color("#F9FAFB") // Light gray
	Background = lipgloss.Color("#111827") // Dark gray

	// Gradient-like colors for progress
	ProgressStart = lipgloss.Color("#22d3ee") // WarpDrop Cyan
	ProgressEnd   = lipgloss.Color("#7C3AED") // Violet
)

// Text styles
var (
	TitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(Primary).
			MarginBottom(1)

	SubtitleStyle = lipgloss.NewStyle().
			Foreground(Secondary).
			Italic(true)

	SuccessStyle = lipgloss.NewStyle().
			Foreground(Success).
			Bold(true)

	ErrorStyle = lipgloss.NewStyle().
			Foreground(Error).
			Bold(true)

	WarningStyle = lipgloss.NewStyle().
			Foreground(Warning)

	MutedStyle = lipgloss.NewStyle().
			Foreground(Muted)

	BoldStyle = lipgloss.NewStyle().
			Bold(true)

	StatusStyle = lipgloss.NewStyle().
			Foreground(Foreground).
			Background(Primary).
			Padding(0, 1).
			Bold(true)
)

// Box styles
var (
	BoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(Primary).
			Padding(1, 2)

	InfoBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(Secondary).
			Padding(1, 2)

	SuccessBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.DoubleBorder()).
			BorderForeground(Success).
			Padding(1, 2)

	ErrorBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.ThickBorder()).
			BorderForeground(Error).
			Padding(1, 2)
)

// Table styles
var (
	TableHeaderStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(Primary).
				BorderBottom(true).
				BorderStyle(lipgloss.NormalBorder()).
				BorderForeground(Muted)

	TableRowStyle = lipgloss.NewStyle().
			Foreground(Foreground)

	TableRowAltStyle = lipgloss.NewStyle().
				Foreground(Muted)

	TableCellStyle = lipgloss.NewStyle().
			Padding(0, 1)
)

// Progress bar styles
var (
	ProgressBarStyle = lipgloss.NewStyle().
				Foreground(Primary)

	ProgressBarFilledStyle = lipgloss.NewStyle().
				Foreground(Success)

	ProgressBarEmptyStyle = lipgloss.NewStyle().
				Foreground(Muted)

	ProgressLabelStyle = lipgloss.NewStyle().
				Foreground(Foreground).
				Width(40)

	ProgressPercentStyle = lipgloss.NewStyle().
				Foreground(Secondary).
				Width(8).
				Align(lipgloss.Right)

	ProgressSpeedStyle = lipgloss.NewStyle().
				Foreground(Muted).
				Width(15).
				Align(lipgloss.Right)
)

// Layout styles
var (
	ContainerStyle = lipgloss.NewStyle().
			Margin(1, 2)

	HeaderStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(Primary).
			Background(lipgloss.Color("#1F2937")).
			Padding(0, 2).
			MarginBottom(1)

	FooterStyle = lipgloss.NewStyle().
			Foreground(Muted).
			MarginTop(1)
)

// Spinner style
var SpinnerStyle = lipgloss.NewStyle().Foreground(Primary)

// Helper function to create styled text
func Styled(text string, style lipgloss.Style) string {
	return style.Render(text)
}

// Emoji helpers for consistent iconography
const (
	IconFile     = "📄"
	IconFolder   = "📁"
	IconSend     = "📤"
	IconReceive  = "📥"
	IconSuccess  = "✅"
	IconError    = "❌"
	IconWarning  = "⚠️"
	IconInfo     = "ℹ️"
	IconLink     = "🔗"
	IconRoom     = "🚪"
	IconPeer     = "👤"
	IconConnect  = "🔌"
	IconSpeed    = "⚡"
	IconTime     = "⏱️"
	IconSize     = "💾"
	IconTransfer = "↔️"
	IconWaiting  = "⏳"
	IconComplete = "🎉"
	IconCopy     = "📋"
	IconWeb      = "🌐"
	IconQR       = "📱"
)
