package common

import (
	"fmt"
	"strings"
)

// NormalizeBasePath ensures a base path starts and ends with "/".
func NormalizeBasePath(p string) string {
	p = strings.TrimSpace(p)
	if p == "" {
		return "/"
	}
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	if !strings.HasSuffix(p, "/") {
		p = p + "/"
	}
	return p
}

// ShellQuote wraps a value in single quotes, escaping embedded single quotes.
func ShellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}

// FormatTraffic formats traffic bytes into human-readable units (B, KB, MB, GB, TB, PB).
func FormatTraffic(trafficBytes int64) string {
	units := []string{"B", "KB", "MB", "GB", "TB", "PB"}
	unitIndex := 0
	size := float64(trafficBytes)

	for size >= 1024 && unitIndex < len(units)-1 {
		size /= 1024
		unitIndex++
	}
	return fmt.Sprintf("%.2f%s", size, units[unitIndex])
}
