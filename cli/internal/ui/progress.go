package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/BioHazard786/Warpdrop/cli/internal/utils"
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
	Started    bool
	Speed      float64
	IsComplete bool
	HasError   bool
	ErrorMsg   string
}

// ProgressModel handles multiple file progress bars
type ProgressModel struct {
	items      []*ProgressItem
	progresses []progress.Model
	width      int
}

// NewProgressModel creates a new multi-file progress model
func NewProgressModel(fileNames []string, fileSizes []int64) ProgressModel {
	items := make([]*ProgressItem, len(fileNames))
	progresses := make([]progress.Model, len(fileNames))

	for i := range fileNames {
		items[i] = &ProgressItem{
			ID:    i,
			Name:  fileNames[i],
			Total: fileSizes[i],
		}

		p := progress.New(
			progress.WithGradient(ProgressStart, ProgressEnd),
			progress.WithWidth(30),
			progress.WithoutPercentage(),
		)
		progresses[i] = p
	}

	return ProgressModel{
		items:      items,
		progresses: progresses,
		width:      80,
	}
}

func (m ProgressModel) Init() tea.Cmd {
	return tickCmd()
}

type TickMsg time.Time

// ProgressMsg updates the progress of a specific file
type ProgressMsg struct {
	ID      int
	Current int64
}

// ProgressCompleteMsg marks a file as complete
type ProgressCompleteMsg struct {
	ID int
}

// ProgressErrorMsg marks a file as errored
type ProgressErrorMsg struct {
	ID  int
	Err error
}

func tickCmd() tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(t time.Time) tea.Msg {
		return TickMsg(t)
	})
}

// AllComplete returns true if all files are complete
func (m ProgressModel) AllComplete() bool {
	for _, item := range m.items {
		if !item.IsComplete && !item.HasError {
			return false
		}
	}
	return true
}

func (m ProgressModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case TickMsg:
		if m.AllComplete() {
			return m, tea.Quit
		}
		return m, tickCmd()

	case tea.KeyMsg:
		if msg.Type == tea.KeyCtrlC {
			return m, tea.Quit
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

	case ProgressMsg:
		if msg.ID >= 0 && msg.ID < len(m.items) {
			item := m.items[msg.ID]
			if !item.Started && msg.Current > 0 {
				item.Started = true
				item.StartTime = time.Now()
			}
			if item.Started {
				elapsed := time.Since(item.StartTime).Seconds()
				if elapsed > 0 {
					item.Speed = float64(msg.Current) / elapsed
				}
			}
			item.Current = msg.Current
			if item.Current >= item.Total {
				item.IsComplete = true
			}
		}
		return m, nil

	case ProgressCompleteMsg:
		if msg.ID >= 0 && msg.ID < len(m.items) {
			m.items[msg.ID].IsComplete = true
			m.items[msg.ID].Current = m.items[msg.ID].Total
		}
		if m.AllComplete() {
			return m, tea.Quit
		}
		return m, nil

	case ProgressErrorMsg:
		if msg.ID >= 0 && msg.ID < len(m.items) {
			m.items[msg.ID].HasError = true
			m.items[msg.ID].ErrorMsg = msg.Err.Error()
		}
		if m.AllComplete() {
			return m, tea.Quit
		}
		return m, nil
	}

	return m, nil
}

func (m ProgressModel) View() string {
	var b strings.Builder

	for i, item := range m.items {
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

		name := utils.TruncateString(item.Name, 30)
		b.WriteString(fmt.Sprintf("%s %s ", icon, nameStyle.Render(name)))

		if item.Total > 0 {
			percent := float64(item.Current) / float64(item.Total)
			b.WriteString(m.progresses[i].ViewAs(percent))
		}

		if item.Total > 0 {
			percent := float64(item.Current) / float64(item.Total) * 100
			b.WriteString(fmt.Sprintf(" %5.1f%%", percent))
		}

		if !item.IsComplete && !item.HasError && item.Speed > 0 {
			b.WriteString(MutedStyle.Render(fmt.Sprintf(" %s", utils.FormatSpeed(item.Speed))))
			remaining := item.Total - item.Current
			if remaining > 0 && item.Speed > 0 {
				etaSeconds := float64(remaining) / item.Speed
				b.WriteString(MutedStyle.Render(fmt.Sprintf(" ETA: %s", utils.FormatTimeDuration(time.Duration(etaSeconds*float64(time.Second))))))
			}
		}

		b.WriteString(MutedStyle.Render(fmt.Sprintf(" (%s/%s)",
			utils.FormatSize(item.Current),
			utils.FormatSize(item.Total))))

		b.WriteString("\n")
	}

	return b.String()
}

// GetTotalProgress returns overall progress information
func (m ProgressModel) GetTotalProgress() (percent float64, current, total int64, speed float64) {
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
