package cliutil

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// AskConfirmation prompts the user for y/n and returns true if they confirm.
func AskConfirmation(prompt string) bool {
	fmt.Printf("%s [y/N]: ", prompt)
	scanner := bufio.NewScanner(os.Stdin)
	if scanner.Scan() {
		answer := strings.TrimSpace(strings.ToLower(scanner.Text()))
		return answer == "y" || answer == "yes"
	}
	return false
}
