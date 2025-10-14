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
	// Parameters returns the JSON schema for the tool's arguments.
	Parameters() any
	// Execute runs the tool with the given arguments and returns the output.
	// The args are expected to be a JSON string.
	Execute(args string) (string, error)
	// RequiresConfirmation indicates whether the tool requires user confirmation before execution.
	RequiresConfirmation() bool
}

// --- ListDirectoryTool ---

// ListDirectoryTool lists the contents of a directory.
type ListDirectoryTool struct{}

func (t *ListDirectoryTool) Name() string {
	return "list_directory"
}

func (t *ListDirectoryTool) RequiresConfirmation() bool {
	return false
}

func (t *ListDirectoryTool) Description() string {
	return "Lists files and subdirectories within a specified directory path. Usage: {\"path\": \"<directory_path>\"}"
}

func (t *ListDirectoryTool) Parameters() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{
				"type":        "string",
				"description": "The path to the directory to list.",
			},
		},
		"required": []string{"path"},
	}
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

// --- ReadFileTool ---

// ReadFileTool reads the content of a file.
type ReadFileTool struct{}

func (t *ReadFileTool) Name() string {
	return "read_file"
}

func (t *ReadFileTool) RequiresConfirmation() bool {
	return false
}

func (t *ReadFileTool) Description() string {
	return "Reads the entire content of a specified file. Usage: {\"path\": \"<file_path>\"}"
}

func (t *ReadFileTool) Parameters() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{
				"type":        "string",
				"description": "The path to the file to read.",
			},
		},
		"required": []string{"path"},
	}
}

type ReadFileArgs struct {
	Path string `json:"path"`
}

func (t *ReadFileTool) Execute(args string) (string, error) {
	var toolArgs ReadFileArgs
	if err := json.Unmarshal([]byte(args), &toolArgs); err != nil {
		return "", fmt.Errorf("invalid arguments for read_file: %w. Expected JSON: {\"path\": \"...\"}", err)
	}

	if toolArgs.Path == "" {
		return "", fmt.Errorf("path argument is required for read_file")
	}

	content, err := os.ReadFile(toolArgs.Path)
	if err != nil {
		return "", fmt.Errorf("error reading file '%s': %w", toolArgs.Path, err)
	}

	return string(content), nil
}

// --- WriteFileTool ---

// WriteFileTool writes content to a specified file.
type WriteFileTool struct{}

func (t *WriteFileTool) Name() string {
	return "write_file"
}

func (t *WriteFileTool) RequiresConfirmation() bool {
	return true
}

func (t *WriteFileTool) Description() string {
	return "Writes content to a specified file, creating the file if it doesn't exist or overwriting it if it does. Usage: {\"path\": \"<file_path>\", \"content\": \"<content_to_write>\"}"
}

func (t *WriteFileTool) Parameters() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{
				"type":        "string",
				"description": "The path to the file to write to.",
			},
			"content": map[string]any{
				"type":        "string",
				"description": "The content to write to the file.",
			},
		},
		"required": []string{"path", "content"},
	}
}

type WriteFileArgs struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

func (t *WriteFileTool) Execute(args string) (string, error) {
	var toolArgs WriteFileArgs
	if err := json.Unmarshal([]byte(args), &toolArgs); err != nil {
		return "", fmt.Errorf("invalid arguments for write_file: %w", err)
	}

	if toolArgs.Path == "" {
		return "", fmt.Errorf("path argument is required for write_file")
	}

	// The permissions 0644 are standard for text files.
	err := os.WriteFile(toolArgs.Path, []byte(toolArgs.Content), 0644)
	if err != nil {
		return "", fmt.Errorf("error writing to file '%s': %w", toolArgs.Path, err)
	}

	return fmt.Sprintf("Successfully wrote %d bytes to %s", len(toolArgs.Content), toolArgs.Path), nil
}
