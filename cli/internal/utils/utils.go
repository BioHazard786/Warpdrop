package utils

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// --- Buffer Management Constants ---
const (
	MinChunkSize     = 4 * 1024        // 4 KB - for very slow connections
	MaxChunkSize     = 64 * 1024       // 64 KB - for fast connections
	DefaultChunkSize = 16 * 1024       // 16 KB - starting size
	HighWaterMark    = 2 * 1024 * 1024 // 2 MB - backpressure threshold
	LowWaterMark     = 512 * 1024      // 512 KB - resume threshold

	// Timeout constants
	SendTimeout   = 60 // seconds - increased for slow connections
	SignalTimeout = 30 // seconds
	DrainTimeout  = 30 // seconds - increased for slow connections
)

// Speed thresholds for chunk size adjustment (in bytes per second)
const (
	SpeedVerySlowThreshold = 50 * 1024       // < 50 KB/s
	SpeedSlowThreshold     = 200 * 1024      // < 200 KB/s
	SpeedMediumThreshold   = 500 * 1024      // < 500 KB/s
	SpeedFastThreshold     = 1 * 1024 * 1024 // < 1 MB/s
	// > 1 MB/s = VERY_FAST
)

// ChunkSizeController manages dynamic chunk sizing based on transfer speed
type ChunkSizeController struct {
	mu               sync.Mutex
	currentChunkSize int
	bytesTransferred int64
	lastUpdateTime   time.Time
	lastSpeed        float64
}

// NewChunkSizeController creates a new chunk size controller
func NewChunkSizeController() *ChunkSizeController {
	return &ChunkSizeController{
		currentChunkSize: DefaultChunkSize,
		lastUpdateTime:   time.Now(),
	}
}

// GetChunkSize returns the current optimal chunk size
func (c *ChunkSizeController) GetChunkSize() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.currentChunkSize
}

// RecordBytesTransferred records bytes transferred and updates chunk size if needed
func (c *ChunkSizeController) RecordBytesTransferred(bytes int64) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.bytesTransferred += bytes

	// Update chunk size every 500ms or after significant data transfer
	elapsed := time.Since(c.lastUpdateTime)
	if elapsed >= 500*time.Millisecond || c.bytesTransferred >= int64(c.currentChunkSize*10) {
		c.updateChunkSize(elapsed)
	}
}

// updateChunkSize calculates and updates the optimal chunk size
func (c *ChunkSizeController) updateChunkSize(elapsed time.Duration) {
	if elapsed <= 0 {
		return
	}

	// Calculate current speed in bytes per second
	currentSpeed := float64(c.bytesTransferred) / elapsed.Seconds()

	// Smooth the speed measurement with exponential moving average
	if c.lastSpeed > 0 {
		c.lastSpeed = c.lastSpeed*0.7 + currentSpeed*0.3
	} else {
		c.lastSpeed = currentSpeed
	}

	// Calculate target chunk size based on speed
	targetChunkSize := c.calculateTargetChunkSize(c.lastSpeed)

	// Smooth transitions: move 25% toward the target to avoid oscillation
	smoothedChunkSize := c.currentChunkSize + int(float64(targetChunkSize-c.currentChunkSize)*0.25)

	// Clamp to valid range
	c.currentChunkSize = max(MinChunkSize, min(MaxChunkSize, smoothedChunkSize))

	// Reset counters
	c.bytesTransferred = 0
	c.lastUpdateTime = time.Now()
}

// calculateTargetChunkSize determines optimal chunk size based on speed
func (c *ChunkSizeController) calculateTargetChunkSize(speed float64) int {
	if speed <= 0 {
		return c.currentChunkSize
	}

	switch {
	case speed < SpeedVerySlowThreshold:
		// Very slow connection (< 50 KB/s): use minimum chunk size
		return MinChunkSize
	case speed < SpeedSlowThreshold:
		// Slow connection (50-200 KB/s): use small chunks
		return 8 * 1024 // 8 KB
	case speed < SpeedMediumThreshold:
		// Medium-slow connection (200-500 KB/s): use medium-small chunks
		return 16 * 1024 // 16 KB
	case speed < SpeedFastThreshold:
		// Medium-fast connection (500 KB/s - 1 MB/s): use medium chunks
		return 32 * 1024 // 32 KB
	default:
		// Fast connection (> 1 MB/s): use large chunks
		return MaxChunkSize // 64 KB
	}
}

// GetSpeed returns the current estimated transfer speed in bytes per second
func (c *ChunkSizeController) GetSpeed() float64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.lastSpeed
}

// FormatSize formats bytes to human readable string
func FormatSize(bytes int64) string {
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

// FormatSpeed formats speed to human readable string
func FormatSpeed(bytesPerSecond float64) string {
	const (
		KB = 1024.0
		MB = KB * 1024
	)

	switch {
	case bytesPerSecond >= MB:
		return fmt.Sprintf("%.2f MB/s", bytesPerSecond/MB)
	case bytesPerSecond >= KB:
		return fmt.Sprintf("%.2f KB/s", bytesPerSecond/KB)
	default:
		return fmt.Sprintf("%.0f B/s", bytesPerSecond)
	}
}

// GetUniqueFilename returns a unique filename by appending (1), (2), etc. if file exists
func GetUniqueFilename(filename string) string {
	// If file doesn't exist, return original name
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		return filename
	}

	// Extract extension and base name
	ext := filepath.Ext(filename)
	nameWithoutExt := filename[:len(filename)-len(ext)]

	// Try appending (1), (2), (3), etc.
	counter := 1
	for {
		newFilename := fmt.Sprintf("%s (%d)%s", nameWithoutExt, counter, ext)
		if _, err := os.Stat(newFilename); os.IsNotExist(err) {
			return newFilename
		}
		counter++
	}
}

// FormatTimeDuration formats duration to human readable string
func FormatTimeDuration(d time.Duration) string {
	seconds := int(d.Seconds()) % 60
	minutes := int(d.Minutes()) % 60
	hours := int(d.Hours())

	if hours > 0 {
		return fmt.Sprintf("%dh %dm %ds", hours, minutes, seconds)
	} else if minutes > 0 {
		return fmt.Sprintf("%dm %ds", minutes, seconds)
	} else {
		return fmt.Sprintf("%ds", seconds)
	}
}
