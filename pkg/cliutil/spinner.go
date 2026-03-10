package cliutil

import (
	"fmt"
	"os"
	"sync"
	"time"
)

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// RunWithSpinner shows a spinner with the given title while fn runs.
// When stdout is not a terminal, it prints the title and runs fn without animation.
func RunWithSpinner(title string, fn func() error) error {
	if !isTerminal() {
		fmt.Println(title)
		return fn()
	}

	var (
		mu      sync.Mutex
		done    bool
		frame   int
		errChan = make(chan error, 1)
	)

	go func() {
		errChan <- fn()
	}()

	// Animate spinner
	go func() {
		for {
			mu.Lock()
			if done {
				mu.Unlock()
				return
			}
			fmt.Printf("\r  %s %s", spinnerFrames[frame%len(spinnerFrames)], title)
			frame++
			mu.Unlock()
			time.Sleep(80 * time.Millisecond)
		}
	}()

	err := <-errChan

	mu.Lock()
	done = true
	mu.Unlock()

	// Clear spinner line
	fmt.Printf("\r\033[K")

	return err
}

// isTerminal returns true if stdout is a terminal.
func isTerminal() bool {
	fi, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}
