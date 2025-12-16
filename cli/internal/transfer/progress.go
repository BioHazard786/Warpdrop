package transfer

import (
	"fmt"
	"time"

	"github.com/BioHazard786/Warpdrop/cli/internal/ui"
	"github.com/BioHazard786/Warpdrop/cli/internal/utils"
	"github.com/BioHazard786/Warpdrop/cli/internal/webrtc"
)

type ProgressTracker struct {
	Model     *ui.ProgressModel
	FileNames []string
	FileSizes []int64
	StartTime int64
}

func NewProgressTracker(fileNames []string, fileSizes []int64) *ProgressTracker {
	return &ProgressTracker{
		Model:     ui.NewProgressModel(fileNames, fileSizes),
		FileNames: fileNames,
		FileSizes: fileSizes,
	}
}

func (p *ProgressTracker) Start() {
	p.StartTime = time.Now().UnixMilli()
}

func (p *ProgressTracker) Update(index int, current int64) {
	if p.Model != nil {
		p.Model.UpdateProgress(index, current)
	}
}

func (p *ProgressTracker) Complete(index int) {
	if p.Model != nil {
		p.Model.MarkComplete(index)
	}
}

func (p *ProgressTracker) Error(index int, msg string) {
	if p.Model != nil {
		p.Model.MarkError(index, msg)
	}
}

func (p *ProgressTracker) View() string {
	if p.Model != nil {
		return p.Model.View()
	}
	return ""
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

func RunProgressLoop(done <-chan struct{}, numFiles int, view func() string, clear func(int)) {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	firstPrint := true
	for {
		select {
		case <-done:
			if !firstPrint {
				clear(numFiles)
			}
			fmt.Print(view())
			return
		case <-ticker.C:
			if !firstPrint {
				clear(numFiles)
			}
			firstPrint = false
			fmt.Print(view())
		}
	}
}

func ClearProgressLines(count int) {
	for range count {
		fmt.Print("\033[A\033[2K")
	}
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
