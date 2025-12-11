package ui

import (
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
)

// SimpleSpinner provides a simple blocking spinner for CLI operations
type SimpleSpinner struct {
	message  string
	spinner  spinner.Spinner
	interval time.Duration
	done     chan struct{}
	stopped  bool
}

// NewSimpleSpinner creates a spinner for general loading operations (Dot style)
func NewSimpleSpinner(message string) *SimpleSpinner {
	return &SimpleSpinner{
		message:  message,
		spinner:  spinner.Dot,
		interval: 80 * time.Millisecond,
		done:     make(chan struct{}),
	}
}

// NewConnectionSpinner creates a spinner for network/connection operations (Globe style)
func NewConnectionSpinner(message string) *SimpleSpinner {
	return &SimpleSpinner{
		message:  message,
		spinner:  spinner.Globe,
		interval: 180 * time.Millisecond,
		done:     make(chan struct{}),
	}
}

// NewWaitingSpinner creates a spinner for waiting on external events (Points style)
func NewWaitingSpinner(message string) *SimpleSpinner {
	return &SimpleSpinner{
		message:  message,
		spinner:  spinner.Points,
		interval: 100 * time.Millisecond,
		done:     make(chan struct{}),
	}
}

func (s *SimpleSpinner) Start() {
	go func() {
		frames := s.spinner.Frames
		i := 0
		for {
			select {
			case <-s.done:
				return
			default:
				frame := SpinnerStyle.Render(frames[i%len(frames)])
				fmt.Printf("\r%s %s", frame, s.message)
				i++
				time.Sleep(s.interval)
			}
		}
	}()
}

func (s *SimpleSpinner) Stop() {
	if !s.stopped {
		s.stopped = true
		close(s.done)
		fmt.Print("\r\033[K") // Clear the line
	}
}

func (s *SimpleSpinner) Success(message string) {
	s.Stop()
	fmt.Printf("%s %s\n", SuccessStyle.Render(IconSuccess), message)
}

func (s *SimpleSpinner) Error(message string) {
	s.Stop()
	fmt.Printf("%s %s\n", ErrorStyle.Render(IconError), message)
}

func (s *SimpleSpinner) UpdateMessage(message string) {
	s.message = message
}

// RunSpinner starts a loading spinner and returns a stop function
func RunSpinner(message string) func() {
	sp := NewSimpleSpinner(message)
	sp.Start()
	return sp.Stop
}

// RunConnectionSpinner starts a connection spinner and returns a stop function
func RunConnectionSpinner(message string) func() {
	sp := NewConnectionSpinner(message)
	sp.Start()
	return sp.Stop
}

// RunWaitingSpinner starts a waiting spinner and returns a stop function
func RunWaitingSpinner(message string) func() {
	sp := NewWaitingSpinner(message)
	sp.Start()
	return sp.Stop
}
