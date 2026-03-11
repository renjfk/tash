package query

import (
	"os/exec"
	"regexp"
	"strings"
)

// CommandExists checks if a command is available in PATH.
func CommandExists(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

// ExtractFirstCommand extracts the first command from a pipeline or compound command.
func ExtractFirstCommand(cmd string) string {
	cmd = strings.TrimSpace(cmd)
	if cmd == "" {
		return ""
	}

	// Handle env vars at the start (VAR=val cmd ...)
	for len(cmd) > 0 {
		spaceIdx := strings.IndexByte(cmd, ' ')
		if spaceIdx < 0 {
			break
		}
		first := cmd[:spaceIdx]
		if !strings.Contains(first, "=") {
			break
		}
		cmd = strings.TrimSpace(cmd[spaceIdx+1:])
	}

	// Split on pipes, semicolons, &&, ||
	re := regexp.MustCompile(`[|;&]`)
	first := re.Split(cmd, 2)[0]
	first = strings.TrimSpace(first)

	parts := strings.Fields(first)
	if len(parts) == 0 {
		return ""
	}
	return parts[0]
}
