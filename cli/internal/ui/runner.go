package ui

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// TransferUI provides a simple interface for managing transfer progress
type TransferUI struct {
	program    *tea.Program
	model      *liveTransferModel
	updateChan chan progressUpdate
	done       chan struct{}
	wg         sync.WaitGroup
}

type progressUpdate struct {
	fileID    int
	current   int64
	completed bool
	failed    bool
	errMsg    string
}

// liveTransferModel is an internal model for live transfer updates
type liveTransferModel struct {
	mode       TransferMode
	state      string
	files      []*liveFileProgress
	progBars   []progress.Model
	spinner    spinner.Model
	startTime  time.Time
	updateChan chan progressUpdate
	mu         sync.RWMutex
	quitting   bool
}

type liveFileProgress struct {
	name      string
	size      int64
	current   int64
	startTime time.Time
	complete  bool
	failed    bool
	errMsg    string
}

// NewTransferUI creates a new transfer UI
func NewTransferUI(mode TransferMode, fileNames []string, fileSizes []int64) *TransferUI {
	updateChan := make(chan progressUpdate, 100)

	files := make([]*liveFileProgress, len(fileNames))
	progBars := make([]progress.Model, len(fileNames))

	for i := range fileNames {
		files[i] = &liveFileProgress{
			name: fileNames[i],
			size: fileSizes[i],
		}
		// Use cyan/blue gradient matching WarpDrop accent color
		progBars[i] = progress.New(
			progress.WithGradient("#22d3ee", "#0ea5e9"), // Cyan to sky blue
			progress.WithWidth(25),
			progress.WithoutPercentage(),
		)
	}

	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = SpinnerStyle

	model := &liveTransferModel{
		mode:       mode,
		state:      "Initializing...",
		files:      files,
		progBars:   progBars,
		spinner:    s,
		updateChan: updateChan,
		startTime:  time.Now(),
	}

	return &TransferUI{
		model:      model,
		updateChan: updateChan,
		done:       make(chan struct{}),
	}
}

// Start starts the UI in a goroutine
func (ui *TransferUI) Start() {
	ui.wg.Add(1)
	go func() {
		defer ui.wg.Done()
		// Don't use any options - default is inline mode without alt screen
		// This keeps previous terminal output visible
		ui.program = tea.NewProgram(ui.model)
		if _, err := ui.program.Run(); err != nil {
			fmt.Printf("UI error: %v\n", err)
		}
	}()
}

// UpdateProgress updates the progress for a specific file
func (ui *TransferUI) UpdateProgress(fileID int, current int64) {
	select {
	case ui.updateChan <- progressUpdate{fileID: fileID, current: current}:
	default:
	}
}

// MarkComplete marks a file as complete
func (ui *TransferUI) MarkComplete(fileID int) {
	select {
	case ui.updateChan <- progressUpdate{fileID: fileID, completed: true}:
	default:
	}
}

// MarkFailed marks a file as failed
func (ui *TransferUI) MarkFailed(fileID int, errMsg string) {
	select {
	case ui.updateChan <- progressUpdate{fileID: fileID, failed: true, errMsg: errMsg}:
	default:
	}
}

// SetState sets the current state message
func (ui *TransferUI) SetState(state string) {
	ui.model.mu.Lock()
	ui.model.state = state
	ui.model.mu.Unlock()
}

// Stop stops the UI
func (ui *TransferUI) Stop() {
	if ui.program != nil {
		ui.program.Quit()
	}
	close(ui.done)
	ui.wg.Wait()
}

// Model methods
func (m *liveTransferModel) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		m.listenForUpdates(),
		tea.Tick(100*time.Millisecond, func(t time.Time) tea.Msg {
			return TickMsg(t)
		}),
	)
}

func (m *liveTransferModel) listenForUpdates() tea.Cmd {
	return func() tea.Msg {
		return <-m.updateChan
	}
}

func (m *liveTransferModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			m.quitting = true
			return m, tea.Quit
		}

	case tea.WindowSizeMsg:
		for i := range m.progBars {
			m.progBars[i].Width = min(25, msg.Width-60)
		}

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		cmds = append(cmds, cmd)

	case TickMsg:
		if !m.quitting && !m.allComplete() {
			cmds = append(cmds, tea.Tick(100*time.Millisecond, func(t time.Time) tea.Msg {
				return TickMsg(t)
			}))
		}

	case progressUpdate:
		m.mu.Lock()
		if msg.fileID >= 0 && msg.fileID < len(m.files) {
			file := m.files[msg.fileID]
			if msg.completed {
				file.complete = true
				file.current = file.size
			} else if msg.failed {
				file.failed = true
				file.errMsg = msg.errMsg
			} else {
				file.current = msg.current
				if file.startTime.IsZero() {
					file.startTime = time.Now()
				}
			}
		}
		m.mu.Unlock()
		cmds = append(cmds, m.listenForUpdates())

	case progress.FrameMsg:
		for i := range m.progBars {
			model, cmd := m.progBars[i].Update(msg)
			m.progBars[i] = model.(progress.Model)
			cmds = append(cmds, cmd)
		}
	}

	return m, tea.Batch(cmds...)
}

func (m *liveTransferModel) allComplete() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for _, f := range m.files {
		if !f.complete && !f.failed {
			return false
		}
	}
	return true
}

func (m *liveTransferModel) View() string {
	if m.quitting {
		return ""
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	var b strings.Builder

	// Header
	modeIcon := IconSend
	modeText := "Sending"
	if m.mode == ModeReceive {
		modeIcon = IconReceive
		modeText = "Receiving"
	}

	b.WriteString(fmt.Sprintf("\n%s %s Files\n\n", modeIcon, modeText))

	// State
	b.WriteString(fmt.Sprintf("%s %s\n\n", m.spinner.View(), m.state))

	// Calculate totals
	var totalSize, totalSent int64
	for _, f := range m.files {
		totalSize += f.size
		totalSent += f.current
	}

	// Overall progress
	var overallPercent float64
	if totalSize > 0 {
		overallPercent = float64(totalSent) / float64(totalSize) * 100
	}

	elapsed := time.Since(m.startTime).Seconds()
	var speed float64
	if elapsed > 0 {
		speed = float64(totalSent) / elapsed
	}

	b.WriteString(fmt.Sprintf("Overall: %s%.1f%%%s (%s/%s) %s\n\n",
		BoldStyle.Render(""),
		overallPercent,
		"",
		formatBytes(totalSent),
		formatBytes(totalSize),
		MutedStyle.Render(formatSpeed(speed)),
	))

	// Per-file progress
	for i, f := range m.files {
		var icon string
		var nameStyle lipgloss.Style

		if f.failed {
			icon = IconError
			nameStyle = ErrorStyle
		} else if f.complete {
			icon = IconSuccess
			nameStyle = SuccessStyle
		} else if f.current > 0 {
			icon = m.spinner.View()
			nameStyle = lipgloss.NewStyle()
		} else {
			icon = "○"
			nameStyle = MutedStyle
		}

		name := truncateString(f.name, 22)
		b.WriteString(fmt.Sprintf("  %s %s ", icon, nameStyle.Width(24).Render(name)))

		// Progress bar
		if f.size > 0 {
			percent := float64(f.current) / float64(f.size)
			b.WriteString(m.progBars[i].ViewAs(percent))
		}

		// Percentage
		if f.size > 0 {
			percent := float64(f.current) / float64(f.size) * 100
			b.WriteString(fmt.Sprintf(" %5.1f%%", percent))
		}

		// Speed and ETA for active files
		if !f.complete && !f.failed && f.current > 0 && !f.startTime.IsZero() {
			fileElapsed := time.Since(f.startTime).Seconds()
			if fileElapsed > 0 {
				fileSpeed := float64(f.current) / fileElapsed
				b.WriteString(MutedStyle.Render(fmt.Sprintf(" %s", formatSpeed(fileSpeed))))
				// Calculate ETA
				remaining := f.size - f.current
				if remaining > 0 && fileSpeed > 0 {
					etaSeconds := float64(remaining) / fileSpeed
					b.WriteString(MutedStyle.Render(fmt.Sprintf(" ETA: %s", formatDuration(etaSeconds))))
				}
			}
		}

		b.WriteString("\n")
	}

	b.WriteString("\n" + MutedStyle.Render("Press q to cancel"))

	return b.String()
}
