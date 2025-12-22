package transfer

import (
	"fmt"
	"time"

	"github.com/BioHazard786/Warpdrop/cli/internal/ui"
	"github.com/BioHazard786/Warpdrop/cli/internal/utils"
	"github.com/BioHazard786/Warpdrop/cli/internal/webrtc"
	tea "github.com/charmbracelet/bubbletea"
)

type ProgressTracker struct {
	Program   *tea.Program
	FileNames []string
	FileSizes []int64
	StartTime int64
}

func NewProgressTracker(fileNames []string, fileSizes []int64) *ProgressTracker {
	model := ui.NewProgressModel(fileNames, fileSizes)
	return &ProgressTracker{
		Program:   tea.NewProgram(model),
		FileNames: fileNames,
		FileSizes: fileSizes,
	}
}

func (p *ProgressTracker) Start() {
	p.StartTime = time.Now().UnixMilli()
}

func (p *ProgressTracker) Run() error {
	_, err := p.Program.Run()
	return err
}

func (p *ProgressTracker) Update(index int, current int64) {
	if p.Program != nil {
		p.Program.Send(ui.ProgressMsg{ID: index, Current: current})
	}
}

func (p *ProgressTracker) Complete(index int) {
	if p.Program != nil {
		p.Program.Send(ui.ProgressCompleteMsg{ID: index})
	}
}

func (p *ProgressTracker) Error(index int, msg string) {
	if p.Program != nil {
		p.Program.Send(ui.ProgressErrorMsg{ID: index, Err: fmt.Errorf("%s", msg)})
	}
}

func (p *ProgressTracker) TotalSize() int64 {
	var total int64
	for _, s := range p.FileSizes {
		total += s
	}
	return total
}

func (p *ProgressTracker) Duration() time.Duration {
	return time.Since(time.UnixMilli(p.StartTime))
}

func RenderSummary(filesCount int, totalSize int64, duration time.Duration) {
	seconds := duration.Seconds()
	fmt.Println()
	ui.RenderTransferSummary(ui.TransferSummary{
		Status:    "✅ Complete",
		Files:     filesCount,
		TotalSize: utils.FormatSize(totalSize),
		Duration:  utils.FormatTimeDuration(duration),
		Speed:     utils.FormatSpeed(float64(totalSize) / seconds),
	})
}

func BuildFileTable(files []webrtc.FileMetadata) []ui.FileTableItem {
	items := make([]ui.FileTableItem, len(files))
	for i, f := range files {
		items[i] = ui.FileTableItem{
			Index: i + 1,
			Name:  f.Name,
			Size:  int64(f.Size),
			Type:  f.Type,
		}
	}
	return items
}

func PromptConsent() bool {
	fmt.Print("\n❓ Do you want to receive these files? [Y/n] ")
	var consent string
	fmt.Scanln(&consent)
	return consent != "n" && consent != "N"
}
