package loop

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func LoadPromptFromFile(path string) (string, error) {
	cleanPath := strings.TrimSpace(path)
	if cleanPath == "" {
		return "", ErrPromptPathEmpty
	}

	ext := strings.ToLower(filepath.Ext(cleanPath))
	if ext != ".md" && ext != ".txt" {
		return "", fmt.Errorf("%w: %s", ErrPromptFileType, cleanPath)
	}

	content, err := os.ReadFile(cleanPath)
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(string(content)), nil
}
