package subprocess

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

// EnsureBinary resolves the binary named `name`. If not found, it prompts the user
// to download it (unless nonInteractive is true, in which case it returns an error).
// fromDir is passed to Download for offline/air-gapped environments.
func EnsureBinary(name, feature, fromDir string, nonInteractive bool) (string, error) {
	path, err := ResolveBinary(name)
	if err == nil {
		return path, nil
	}

	if nonInteractive {
		return "", fmt.Errorf("%s is required for %s but was not found (non-interactive mode)", name, feature)
	}

	fmt.Printf("%s is required for %s. Download it now? [Y/n] ", name, feature)
	reader := bufio.NewReader(os.Stdin)
	answer, _ := reader.ReadString('\n')
	answer = strings.TrimSpace(strings.ToLower(answer))

	if answer != "" && answer != "y" && answer != "yes" {
		return "", fmt.Errorf("%s is unavailable — %s feature disabled", name, feature)
	}

	fmt.Printf("Downloading %s...\n", name)
	path, err = Download(name, fromDir)
	if err != nil {
		return "", fmt.Errorf("download %s: %w", name, err)
	}
	fmt.Printf("%s installed to %s\n", name, path)
	return path, nil
}
