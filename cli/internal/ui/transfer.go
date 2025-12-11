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

// TransferState represents the current state of the transfer
type TransferState int

const (
	StateIdle TransferState = iota
	StateConnecting
	StateWaitingForPeer
	StateWaitingForAccept
	StateTransferring
	StateComplete
	StateError
	StateCancelled
)

// TransferMode represents send or receive
type TransferMode int

const (
	ModeSend TransferMode = iota
	ModeReceive
)

// FileProgress tracks individual file progress
type FileProgress struct {
	ID         int
	Name       string
	Size       int64
	Sent       int64
	Speed      float64
	StartTime  time.Time
	IsComplete bool
	HasError   bool
	ErrorMsg   string
}

// TransferModel is the main Bubble Tea model for file transfers
type TransferModel struct {
	// Mode
	mode TransferMode

	// State
	state    TransferState
	stateMsg string

	// Room info
	roomID   string
	roomLink string
	peerType string

	// Files
	files    []*FileProgress
	progress []progress.Model

	// Spinner for waiting states
	spinner spinner.Model

	// Totals
	totalSize    int64
	totalSent    int64
	startTime    time.Time
	overallSpeed float64

	// UI
	width  int
	height int

	// Synchronization
	mu sync.RWMutex

	// Channel for external updates
	updateChan chan TransferUpdate

	// Done channel
	done chan struct{}

	// Error
	err error
}

// TransferUpdate is a message sent from external goroutines to update the UI
type TransferUpdate struct {
	Type    UpdateType
	FileID  int
	Bytes   int64
	Message string
	Error   error
}

type UpdateType int

const (
	UpdateProgress UpdateType = iota
	UpdateFileComplete
	UpdateFileError
	UpdateState
	UpdateRoomCreated
	UpdatePeerJoined
	UpdateComplete
	UpdateError
)

// NewTransferModel creates a new transfer model
func NewTransferModel(mode TransferMode) *TransferModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = SpinnerStyle

	return &TransferModel{
		mode:       mode,
		state:      StateIdle,
		spinner:    s,
		files:      make([]*FileProgress, 0),
		progress:   make([]progress.Model, 0),
		updateChan: make(chan TransferUpdate, 100),
		done:       make(chan struct{}),
		width:      80,
		height:     24,
	}
}

// SetFiles sets the files to transfer
func (m *TransferModel) SetFiles(names []string, sizes []int64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.files = make([]*FileProgress, len(names))
	m.progress = make([]progress.Model, len(names))
	m.totalSize = 0

	for i := range names {
		m.files[i] = &FileProgress{
			ID:   i,
			Name: names[i],
			Size: sizes[i],
		}
		m.totalSize += sizes[i]

		p := progress.New(
			progress.WithDefaultGradient(),
			progress.WithWidth(25),
			progress.WithoutPercentage(),
		)
		m.progress[i] = p
	}
}

// GetUpdateChannel returns the channel for sending updates
func (m *TransferModel) GetUpdateChannel() chan<- TransferUpdate {
	return m.updateChan
}

// SetState sets the current state
func (m *TransferModel) SetState(state TransferState, msg string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.state = state
	m.stateMsg = msg
}

// SetRoomInfo sets room information
func (m *TransferModel) SetRoomInfo(roomID, roomLink string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.roomID = roomID
	m.roomLink = roomLink
}

// SetPeerType sets the peer type
func (m *TransferModel) SetPeerType(peerType string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.peerType = peerType
}

func (m *TransferModel) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		m.waitForUpdates(),
		tickCmd(),
	)
}

// waitForUpdates returns a command that listens for external updates
func (m *TransferModel) waitForUpdates() tea.Cmd {
	return func() tea.Msg {
		select {
		case update := <-m.updateChan:
			return update
		case <-m.done:
			return nil
		}
	}
}

func (m *TransferModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		for i := range m.progress {
			m.progress[i].Width = min(25, msg.Width-60)
		}

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		cmds = append(cmds, cmd)

	case TickMsg:
		// Update speed calculations
		m.updateSpeeds()
		if m.state != StateComplete && m.state != StateError {
			cmds = append(cmds, tickCmd())
		}

	case TransferUpdate:
		m.handleUpdate(msg)
		cmds = append(cmds, m.waitForUpdates())

	case progress.FrameMsg:
		for i := range m.progress {
			model, cmd := m.progress[i].Update(msg)
			m.progress[i] = model.(progress.Model)
			cmds = append(cmds, cmd)
		}
	}

	return m, tea.Batch(cmds...)
}

func (m *TransferModel) handleUpdate(update TransferUpdate) {
	m.mu.Lock()
	defer m.mu.Unlock()

	switch update.Type {
	case UpdateProgress:
		if update.FileID >= 0 && update.FileID < len(m.files) {
			file := m.files[update.FileID]
			file.Sent = update.Bytes
			if file.StartTime.IsZero() {
				file.StartTime = time.Now()
			}
		}

	case UpdateFileComplete:
		if update.FileID >= 0 && update.FileID < len(m.files) {
			m.files[update.FileID].IsComplete = true
			m.files[update.FileID].Sent = m.files[update.FileID].Size
		}

	case UpdateFileError:
		if update.FileID >= 0 && update.FileID < len(m.files) {
			m.files[update.FileID].HasError = true
			m.files[update.FileID].ErrorMsg = update.Message
		}

	case UpdateState:
		m.stateMsg = update.Message

	case UpdateRoomCreated:
		m.state = StateWaitingForPeer
		parts := strings.Split(update.Message, "|")
		if len(parts) >= 2 {
			m.roomID = parts[0]
			m.roomLink = parts[1]
		}

	case UpdatePeerJoined:
		m.peerType = update.Message
		m.state = StateWaitingForAccept

	case UpdateComplete:
		m.state = StateComplete

	case UpdateError:
		m.state = StateError
		m.err = update.Error
	}
}

func (m *TransferModel) updateSpeeds() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.totalSent = 0
	for _, file := range m.files {
		m.totalSent += file.Sent
		if !file.StartTime.IsZero() && !file.IsComplete {
			elapsed := time.Since(file.StartTime).Seconds()
			if elapsed > 0 {
				file.Speed = float64(file.Sent) / elapsed
			}
		}
	}

	if !m.startTime.IsZero() {
		elapsed := time.Since(m.startTime).Seconds()
		if elapsed > 0 {
			m.overallSpeed = float64(m.totalSent) / elapsed
		}
	}
}

func (m *TransferModel) View() string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var b strings.Builder

	// Header
	var modeIcon, modeText string
	if m.mode == ModeSend {
		modeIcon = IconSend
		modeText = "Sending Files"
	} else {
		modeIcon = IconReceive
		modeText = "Receiving Files"
	}

	header := HeaderStyle.Render(fmt.Sprintf("%s WarpDrop - %s", modeIcon, modeText))
	b.WriteString(header + "\n\n")

	// State-specific content
	switch m.state {
	case StateIdle, StateConnecting:
		b.WriteString(m.viewConnecting())

	case StateWaitingForPeer:
		b.WriteString(m.viewWaitingForPeer())

	case StateWaitingForAccept:
		b.WriteString(m.viewWaitingForAccept())

	case StateTransferring:
		b.WriteString(m.viewTransferring())

	case StateComplete:
		b.WriteString(m.viewComplete())

	case StateError:
		b.WriteString(m.viewError())

	case StateCancelled:
		b.WriteString(m.viewCancelled())
	}

	// Footer
	footer := m.viewFooter()
	b.WriteString("\n" + footer)

	return ContainerStyle.Render(b.String())
}

func (m *TransferModel) viewConnecting() string {
	return fmt.Sprintf("%s %s",
		m.spinner.View(),
		m.stateMsg,
	)
}

func (m *TransferModel) viewWaitingForPeer() string {
	var b strings.Builder

	// Room info box
	if m.roomID != "" {
		roomInfo := NewRoomInfo(m.roomID, m.roomLink)
		b.WriteString(roomInfo.View())
		b.WriteString("\n\n")
	}

	// Waiting spinner
	b.WriteString(fmt.Sprintf("%s Waiting for receiver to join...",
		m.spinner.View(),
	))

	return b.String()
}

func (m *TransferModel) viewWaitingForAccept() string {
	var b strings.Builder

	// Peer joined notification
	b.WriteString(SuccessStyle.Render(fmt.Sprintf("%s Peer joined!", IconPeer)))
	if m.peerType != "" {
		b.WriteString(MutedStyle.Render(fmt.Sprintf(" (type: %s)", m.peerType)))
	}
	b.WriteString("\n\n")

	// Waiting for acceptance
	b.WriteString(fmt.Sprintf("%s Waiting for receiver to accept files...",
		m.spinner.View(),
	))

	return b.String()
}

func (m *TransferModel) viewTransferring() string {
	var b strings.Builder

	// Overall progress
	var overallPercent float64
	if m.totalSize > 0 {
		overallPercent = float64(m.totalSent) / float64(m.totalSize) * 100
	}

	b.WriteString(fmt.Sprintf("%s Overall: %.1f%% (%s/%s) %s\n\n",
		IconTransfer,
		overallPercent,
		formatBytes(m.totalSent),
		formatBytes(m.totalSize),
		MutedStyle.Render(formatSpeed(m.overallSpeed)),
	))

	// Per-file progress
	for i, file := range m.files {
		var icon string
		var nameStyle lipgloss.Style

		if file.HasError {
			icon = IconError
			nameStyle = ErrorStyle
		} else if file.IsComplete {
			icon = IconSuccess
			nameStyle = SuccessStyle
		} else if file.Sent > 0 {
			icon = m.spinner.View()
			nameStyle = lipgloss.NewStyle()
		} else {
			icon = "○"
			nameStyle = MutedStyle
		}

		// File name
		name := truncateString(file.Name, 25)
		b.WriteString(fmt.Sprintf("  %s %s ", icon, nameStyle.Width(27).Render(name)))

		// Progress bar
		if file.Size > 0 {
			percent := float64(file.Sent) / float64(file.Size)
			b.WriteString(m.progress[i].ViewAs(percent))
		}

		// Percentage
		if file.Size > 0 {
			percent := float64(file.Sent) / float64(file.Size) * 100
			b.WriteString(fmt.Sprintf(" %5.1f%%", percent))
		}

		// Speed (only for active files)
		if !file.IsComplete && !file.HasError && file.Speed > 0 {
			b.WriteString(MutedStyle.Render(fmt.Sprintf(" %s", formatSpeed(file.Speed))))
		}

		b.WriteString("\n")
	}

	return b.String()
}

func (m *TransferModel) viewComplete() string {
	var b strings.Builder

	// Success message
	b.WriteString(SuccessStyle.Render(fmt.Sprintf("%s Transfer Complete!", IconComplete)))
	b.WriteString("\n\n")

	// Summary
	elapsed := time.Since(m.startTime)
	summary := NewSummary("Transfer Summary", SummaryStatusSuccess).
		AddItem("Status", "Complete").
		AddItem("Files", fmt.Sprintf("%d", len(m.files))).
		AddItem("Total Size", formatBytes(m.totalSize)).
		AddItem("Time", fmt.Sprintf("%.2f seconds", elapsed.Seconds())).
		AddItem("Avg Speed", formatSpeed(m.overallSpeed))

	b.WriteString(summary.View())

	return b.String()
}

func (m *TransferModel) viewError() string {
	var b strings.Builder

	b.WriteString(ErrorStyle.Render(fmt.Sprintf("%s Transfer Failed", IconError)))
	b.WriteString("\n\n")

	if m.err != nil {
		errorBox := ErrorBoxStyle.Render(m.err.Error())
		b.WriteString(errorBox)
	}

	return b.String()
}

func (m *TransferModel) viewCancelled() string {
	return WarningStyle.Render(fmt.Sprintf("%s Transfer cancelled by user", IconWarning))
}

func (m *TransferModel) viewFooter() string {
	if m.state == StateComplete || m.state == StateError {
		return MutedStyle.Render("Press 'q' to exit")
	}
	return MutedStyle.Render("Press 'q' or Ctrl+C to cancel")
}

// StartTransfer marks the beginning of file transfer
func (m *TransferModel) StartTransfer() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.state = StateTransferring
	m.startTime = time.Now()
	for _, file := range m.files {
		file.StartTime = time.Now()
	}
}

// Close closes the model
func (m *TransferModel) Close() {
	close(m.done)
}

// IsComplete returns true if transfer is complete
func (m *TransferModel) IsComplete() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.state == StateComplete
}

// HasError returns true if there was an error
func (m *TransferModel) HasError() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.state == StateError
}
