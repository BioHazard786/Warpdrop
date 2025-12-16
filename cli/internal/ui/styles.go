package ui

import (
	"fmt"

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
	ProgressStart = "#22d3ee" // WarpDrop Cyan
	ProgressEnd   = "#0ea5e9" // Sky Blue
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
				Align(lipgloss.Center)

	tableCellStyle = lipgloss.NewStyle().Padding(0, 1)

	TableRowStyle = tableCellStyle.Foreground(lipgloss.Color("255"))

	TableRowAltStyle = tableCellStyle.Foreground(lipgloss.Color("245"))
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

func PrintError(msg string) {
	fmt.Printf("%s %s\n", ErrorStyle.Render(IconError), ErrorStyle.Render(msg))
}

func PrintErrorf(format string, args ...any) {
	PrintError(fmt.Sprintf(format, args...))
}

func PrintWarning(msg string) {
	fmt.Printf("%s %s\n", WarningStyle.Render(IconWarning), WarningStyle.Render(msg))
}

func PrintWarningf(format string, args ...any) {
	PrintWarning(fmt.Sprintf(format, args...))
}

func PrintSuccess(msg string) {
	fmt.Printf("%s %s\n", SuccessStyle.Render(IconSuccess), msg)
}

func PrintSuccessf(format string, args ...any) {
	PrintSuccess(fmt.Sprintf(format, args...))
}

func PrintInfo(msg string) {
	fmt.Printf("%s %s\n", IconInfo, msg)
}

func PrintInfof(format string, args ...any) {
	PrintInfo(fmt.Sprintf(format, args...))
}

func FormatError(err error) string {
	return fmt.Sprintf("%s %s", ErrorStyle.Render(IconError), ErrorStyle.Render(err.Error()))
}
