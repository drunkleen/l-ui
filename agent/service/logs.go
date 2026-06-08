package service

import (
	"fmt"
	"os"
	"path/filepath"
)

type LogService struct{}

func NewLogService() *LogService {
	return &LogService{}
}

func (s *LogService) TailLog(path string, tailLines int) ([]string, error) {
	if tailLines <= 0 {
		tailLines = 50
	}
	if tailLines > 500 {
		tailLines = 500
	}

	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return nil, fmt.Errorf("read log: %w", err)
	}

	lines := splitLines(string(data))
	if len(lines) > tailLines {
		lines = lines[len(lines)-tailLines:]
	}
	return lines, nil
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}
