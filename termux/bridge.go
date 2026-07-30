package termux

import (
	"os"
	"os/exec"
	"strings"
)

// IsTermux detects if we're running inside Termux
func IsTermux() bool {
	prefix := os.Getenv("PREFIX")
	return strings.Contains(prefix, "com.termux")
}

// ShareFile opens Android share sheet for a file
func ShareFile(path string) error {
	return exec.Command("termux-share", path).Run()
}

// CopyToClipboard copies text to Android clipboard
func CopyToClipboard(text string) error {
	cmd := exec.Command("termux-clipboard-set")
	cmd.Stdin = strings.NewReader(text)
	return cmd.Run()
}

// HasStorageAccess checks if Termux has shared storage permission
func HasStorageAccess() bool {
	_, err := os.Stat("/sdcard")
	return err == nil
}

// OpenURL opens a URL in the default Android browser
func OpenURL(url string) error {
	// Try termux-open-url first (more reliable), fall back to xdg-open
	if err := exec.Command("termux-open-url", url).Start(); err != nil {
		return exec.Command("xdg-open", url).Start()
	}
	return nil
}

// RunCommand executes a shell command and returns stdout+stderr
func RunCommand(command string) (string, error) {
	cmd := exec.Command("bash", "-c", command)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// Vibrate triggers haptic feedback (Termux API)
func Vibrate(ms int) {
	if IsTermux() {
		exec.Command("termux-vibrate", "-d", strings.TrimSpace(
			strings.Replace(strings.TrimSpace(strings.TrimRight(
				strings.Repeat("0", ms/100), "0")), "", "0", 1),
		)).Run()
	}
}
