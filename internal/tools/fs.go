package tools

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// Tool represents a function that can be called by the agent.
type Tool interface {
	// Name is the name of the tool, as it would be called by the model.
	Name() string
	// Description is a description of the tool's purpose, inputs, and outputs.
	Description() string
	// Execute runs the tool with the given arguments and returns the output.
	// The args are expected to be a JSON string.
	Execute(args string) (string, error)
}

// --- ListDirectoryTool ---

// ListDirectoryTool lists the contents of a directory.
type ListDirectoryTool struct{}

func (t *ListDirectoryTool) Name() string {
	return "list_directory"
}

func (t *ListDirectoryTool) Description() string {
	return "Lists files and subdirectories within a specified directory path. Usage: {\"path\": \"<directory_path>\"}"
}

type ListDirectoryArgs struct {
	Path string `json:"path"`
}

func (t *ListDirectoryTool) Execute(args string) (string, error) {
	var toolArgs ListDirectoryArgs
	if err := json.Unmarshal([]byte(args), &toolArgs); err != nil {
		return "", fmt.Errorf("invalid arguments for list_directory: %w. Expected JSON: {\"path\": \"...\"}", err)
	}

	path := toolArgs.Path
	if path == "" {
		path = "." // Default to current directory
	}

	entries, err := os.ReadDir(path)
	if err != nil {
		return "", fmt.Errorf("error reading directory '%s': %w", path, err)
	}

	var output strings.Builder
	output.WriteString(fmt.Sprintf("Contents of %s:\n", path))

	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			continue // Skip files we can't get info for
		}

		mode := info.Mode()
		size := info.Size()
		modTime := info.ModTime().Format("2006-01-02 15:04:05")
		name := entry.Name()

		if entry.IsDir() {
			name += "/"
		}

		// Format: permissions size modification_time name
		output.WriteString(fmt.Sprintf("% -12s % -10d %s %s\n", mode.String(), size, modTime, name))
	}

	return output.String(), nil
}
