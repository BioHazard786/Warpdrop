package ui

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/bubbles/progress"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ProgressItem represents a single file transfer progress
type ProgressItem struct {
	ID         int
	Name       string
	Total      int64
	Current    int64
	StartTime  time.Time
	Started    bool    // tracks if transfer has actually started
	Speed      float64 // bytes per second
	IsComplete bool
	HasError   bool
	ErrorMsg   string
}

// ProgressModel handles multiple file progress bars
type ProgressModel struct {
	items      []*ProgressItem
	progresses []progress.Model
	width      int
	mu         sync.RWMutex
}

// NewProgressModel creates a new multi-file progress model
func NewProgressModel(fileNames []string, fileSizes []int64) *ProgressModel {
	items := make([]*ProgressItem, len(fileNames))
	progresses := make([]progress.Model, len(fileNames))

	for i := range fileNames {
		items[i] = &ProgressItem{
			ID:      i,
			Name:    fileNames[i],
			Total:   fileSizes[i],
			Current: 0,
			Started: false, // StartTime will be set on first byte received
		}

		// Use cyan/blue gradient matching WarpDrop accent color
		p := progress.New(
			progress.WithGradient("#22d3ee", "#0ea5e9"), // Cyan to sky blue
			progress.WithWidth(30),
			progress.WithoutPercentage(),
		)
		progresses[i] = p
	}

	return &ProgressModel{
		items:      items,
		progresses: progresses,
		width:      80,
	}
}

func (m *ProgressModel) Init() tea.Cmd {
	return tickCmd()
}

// TickMsg is sent periodically to update the progress display
type TickMsg time.Time

func tickCmd() tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(t time.Time) tea.Msg {
		return TickMsg(t)
	})
}

// UpdateProgress updates a specific file's progress
func (m *ProgressModel) UpdateProgress(id int, current int64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if id >= 0 && id < len(m.items) {
		item := m.items[id]
		// Start timing from first byte received, not from model creation
		if !item.Started && current > 0 {
			item.Started = true
			item.StartTime = time.Now()
		}
		if item.Started {
			elapsed := time.Since(item.StartTime).Seconds()
			if elapsed > 0 {
				item.Speed = float64(current) / elapsed
			}
		}
		item.Current = current
		if current >= item.Total {
			item.IsComplete = true
		}
	}
}

// MarkComplete marks a file as complete
func (m *ProgressModel) MarkComplete(id int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if id >= 0 && id < len(m.items) {
		m.items[id].IsComplete = true
		m.items[id].Current = m.items[id].Total
	}
}

// MarkError marks a file as having an error
func (m *ProgressModel) MarkError(id int, errMsg string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if id >= 0 && id < len(m.items) {
		m.items[id].HasError = true
		m.items[id].ErrorMsg = errMsg
	}
}

// AllComplete returns true if all files are complete
func (m *ProgressModel) AllComplete() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, item := range m.items {
		if !item.IsComplete && !item.HasError {
			return false
		}
	}
	return true
}

func (m *ProgressModel) Update(msg tea.Msg) (*ProgressModel, tea.Cmd) {
	switch msg := msg.(type) {
	case TickMsg:
		// Continue ticking if not all complete
		if !m.AllComplete() {
			return m, tickCmd()
		}
		return m, nil

	case tea.WindowSizeMsg:
		m.width = msg.Width
		for i := range m.progresses {
			m.progresses[i].Width = min(30, msg.Width-50)
		}
		return m, nil

	case progress.FrameMsg:
		var cmds []tea.Cmd
		for i := range m.progresses {
			var cmd tea.Cmd
			newModel, cmd := m.progresses[i].Update(msg)
			m.progresses[i] = newModel.(progress.Model)
			cmds = append(cmds, cmd)
		}
		return m, tea.Batch(cmds...)
	}

	return m, nil
}

func (m *ProgressModel) View() string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var b strings.Builder

	for i, item := range m.items {
		// Status icon
		var icon string
		var nameStyle lipgloss.Style

		if item.HasError {
			icon = IconError
			nameStyle = ErrorStyle
		} else if item.IsComplete {
			icon = IconSuccess
			nameStyle = SuccessStyle
		} else {
			icon = IconFile
			nameStyle = lipgloss.NewStyle()
		}

		// File name (truncated if needed)
		name := truncateString(item.Name, 30)
		b.WriteString(fmt.Sprintf("%s %s ", icon, nameStyle.Render(name)))

		// Progress bar
		if item.Total > 0 {
			percent := float64(item.Current) / float64(item.Total)
			b.WriteString(m.progresses[i].ViewAs(percent))
		}

		// Percentage
		if item.Total > 0 {
			percent := float64(item.Current) / float64(item.Total) * 100
			b.WriteString(fmt.Sprintf(" %5.1f%%", percent))
		}

		// Speed and ETA
		if !item.IsComplete && !item.HasError && item.Speed > 0 {
			b.WriteString(MutedStyle.Render(fmt.Sprintf(" %s", formatSpeed(item.Speed))))
			// Calculate ETA
			remaining := item.Total - item.Current
			if remaining > 0 && item.Speed > 0 {
				etaSeconds := float64(remaining) / item.Speed
				b.WriteString(MutedStyle.Render(fmt.Sprintf(" ETA: %s", formatDuration(etaSeconds))))
			}
		}

		// Size
		b.WriteString(MutedStyle.Render(fmt.Sprintf(" (%s/%s)",
			formatBytes(item.Current),
			formatBytes(item.Total))))

		b.WriteString("\n")
	}

	return b.String()
}

// GetTotalProgress returns overall progress percentage and total stats
func (m *ProgressModel) GetTotalProgress() (percent float64, current, total int64, speed float64) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var totalSpeed float64
	for _, item := range m.items {
		current += item.Current
		total += item.Total
		if !item.IsComplete {
			totalSpeed += item.Speed
		}
	}

	if total > 0 {
		percent = float64(current) / float64(total) * 100
	}

	return percent, current, total, totalSpeed
}

// SingleProgressBar is a simpler single progress bar for overall transfer
type SingleProgressBar struct {
	progress  progress.Model
	current   int64
	total     int64
	startTime time.Time
	label     string
	mu        sync.RWMutex
}

func NewSingleProgressBar(label string, total int64) *SingleProgressBar {
	p := progress.New(
		progress.WithDefaultGradient(),
		progress.WithWidth(40),
	)

	return &SingleProgressBar{
		progress:  p,
		total:     total,
		startTime: time.Now(),
		label:     label,
	}
}

func (p *SingleProgressBar) Update(current int64) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.current = current
}

func (p *SingleProgressBar) View() string {
	p.mu.RLock()
	defer p.mu.RUnlock()

	var percent float64
	if p.total > 0 {
		percent = float64(p.current) / float64(p.total)
	}

	elapsed := time.Since(p.startTime).Seconds()
	var speed float64
	if elapsed > 0 {
		speed = float64(p.current) / elapsed
	}

	return fmt.Sprintf("%s %s %s %s",
		p.label,
		p.progress.ViewAs(percent),
		MutedStyle.Render(formatSpeed(speed)),
		MutedStyle.Render(fmt.Sprintf("(%s/%s)", formatBytes(p.current), formatBytes(p.total))),
	)
}

// Helper functions
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

func formatBytes(bytes int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
	)

	switch {
	case bytes >= GB:
		return fmt.Sprintf("%.2f GB", float64(bytes)/float64(GB))
	case bytes >= MB:
		return fmt.Sprintf("%.2f MB", float64(bytes)/float64(MB))
	case bytes >= KB:
		return fmt.Sprintf("%.2f KB", float64(bytes)/float64(KB))
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}

func formatSpeed(bytesPerSecond float64) string {
	const (
		KB = 1024.0
		MB = KB * 1024.0
		GB = MB * 1024.0
	)

	switch {
	case bytesPerSecond >= GB:
		return fmt.Sprintf("%.2f GB/s", bytesPerSecond/GB)
	case bytesPerSecond >= MB:
		return fmt.Sprintf("%.2f MB/s", bytesPerSecond/MB)
	case bytesPerSecond >= KB:
		return fmt.Sprintf("%.2f KB/s", bytesPerSecond/KB)
	default:
		return fmt.Sprintf("%.0f B/s", bytesPerSecond)
	}
}

func formatDuration(seconds float64) string {
	if seconds < 1 {
		return "<1s"
	}
	if seconds < 60 {
		return fmt.Sprintf("%.0fs", seconds)
	}
	if seconds < 3600 {
		mins := int(seconds) / 60
		secs := int(seconds) % 60
		return fmt.Sprintf("%dm%ds", mins, secs)
	}
	hours := int(seconds) / 3600
	mins := (int(seconds) % 3600) / 60
	return fmt.Sprintf("%dh%dm", hours, mins)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
