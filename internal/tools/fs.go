package tools

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
)

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
		output.WriteString(fmt.Sprintf("%-12s %-10d %s %s\n", mode.String(), size, modTime, name))
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

// --- SearchFileContentTool ---

// SearchFileContentTool searches for a pattern in files within a directory.
type SearchFileContentTool struct{}

func (t *SearchFileContentTool) Name() string {
	return "search_file_content"
}

func (t *SearchFileContentTool) RequiresConfirmation() bool {
	return false
}

func (t *SearchFileContentTool) Description() string {
	return "Recursively searches for a regular expression pattern in files within a directory. Usage: {\"path\": \"<directory_path>\", \"pattern\": \"<regex_pattern>\"}"
}

func (t *SearchFileContentTool) Parameters() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{
				"type":        "string",
				"description": "The directory path to start searching from.",
			},
			"pattern": map[string]any{
				"type":        "string",
				"description": "The regular expression pattern to search for.",
			},
		},
		"required": []string{"path", "pattern"},
	}
}

type SearchFileContentArgs struct {
	Path    string `json:"path"`
	Pattern string `json:"pattern"`
}

func (t *SearchFileContentTool) Execute(args string) (string, error) {
	var toolArgs SearchFileContentArgs
	if err := json.Unmarshal([]byte(args), &toolArgs); err != nil {
		return "", fmt.Errorf("invalid arguments for search_file_content: %w", err)
	}

	if toolArgs.Path == "" || toolArgs.Pattern == "" {
		return "", fmt.Errorf("path and pattern arguments are required for search_file_content")
	}

	regex, err := regexp.Compile(toolArgs.Pattern)
	if err != nil {
		return "", fmt.Errorf("invalid regex pattern: %w", err)
	}

	var results strings.Builder
	var matchesFound int

	err = filepath.WalkDir(toolArgs.Path, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err // Propagate errors from WalkDir
		}
		if !d.IsDir() {
			file, err := os.Open(path)
			if err != nil {
				// Can't open, just log it and continue
				results.WriteString(fmt.Sprintf("Could not open file %s: %v\n", path, err))
				return nil
			}
			defer file.Close()

			scanner := bufio.NewScanner(file)
			lineNumber := 1
			for scanner.Scan() {
				if regex.MatchString(scanner.Text()) {
					matchesFound++
					results.WriteString(fmt.Sprintf("%s:%d: %s\n", path, lineNumber, scanner.Text()))
				}
				lineNumber++
			}
		}
		return nil
	})

	if err != nil {
		return "", fmt.Errorf("error walking directory '%s': %w", toolArgs.Path, err)
	}

	if matchesFound == 0 {
		return "No matches found.", nil
	}

	return results.String(), nil
}

// --- GlobTool ---

// GlobTool finds files matching a glob pattern.
type GlobTool struct{}

func (t *GlobTool) Name() string {
	return "glob"
}

func (t *GlobTool) RequiresConfirmation() bool {
	return false
}

func (t *GlobTool) Description() string {
	return "Finds files and directories matching a specified glob pattern within a given path. Usage: {\"pattern\": \"<glob_pattern>\", \"path\": \"<base_directory>\"}"
}

func (t *GlobTool) Parameters() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"pattern": map[string]any{
				"type":        "string",
				"description": "The glob pattern to match files against (e.g., \"internal/**/*.go\").",
			},
			"path": map[string]any{
				"type":        "string",
				"description": "Optional: The base directory to start the glob search from. Defaults to the current working directory if not provided.",
			},
		},
		"required": []string{"pattern"},
	}
}

type GlobArgs struct {
	Pattern string `json:"pattern"`
	Path    string `json:"path"`
}

func (t *GlobTool) Execute(args string) (string, error) {
	var toolArgs GlobArgs
	if err := json.Unmarshal([]byte(args), &toolArgs); err != nil {
		return "", fmt.Errorf("invalid arguments for glob: %w", err)
	}

	if toolArgs.Pattern == "" {
		return "", fmt.Errorf("pattern argument is required for glob")
	}

	basePath := toolArgs.Path
	if basePath == "" {
		basePath = "."
	}

	var matches []string
	err := filepath.WalkDir(basePath, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			return nil // Skip directories, we only care about files matching the pattern
		}

		// Get the path relative to the basePath for matching against the pattern
		relativePath, err := filepath.Rel(basePath, path)
		if err != nil {
			return fmt.Errorf("failed to get relative path for %s: %w", path, err)
		}

		matched, err := doublestar.Match(toolArgs.Pattern, relativePath)
		if err != nil {
			// This error indicates a malformed pattern, not a non-match
			return fmt.Errorf("invalid glob pattern %s: %w", toolArgs.Pattern, err)
		}

		if matched {
			matches = append(matches, path)
		}

		return nil
	})

	if err != nil {
		return "", fmt.Errorf("error walking directory '%s': %w", basePath, err)
	}

	if len(matches) == 0 {
		return "No files matched the pattern.", nil
	}

	return strings.Join(matches, "\n"), nil
}

// --- ReplaceTool ---

// ReplaceTool replaces the first occurrence of a string in a file.
type ReplaceTool struct{}

func (t *ReplaceTool) Name() string {
	return "replace"
}

func (t *ReplaceTool) RequiresConfirmation() bool {
	return true // Requires user confirmation as it modifies a file
}

func (t *ReplaceTool) Description() string {
	return "Replaces the first occurrence of a specified old string with a new string in a file. Usage: {\"path\": \"<file_path>\", \"old_string\": \"<string_to_find>\", \"new_string\": \"<string_to_replace_with>\"}"
}

func (t *ReplaceTool) Parameters() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{
				"type":        "string",
				"description": "The path to the file to modify.",
			},
			"old_string": map[string]any{
				"type":        "string",
				"description": "The string to find in the file.",
			},
			"new_string": map[string]any{
				"type":        "string",
				"description": "The string to replace the old string with.",
			},
		},
		"required": []string{"path", "old_string", "new_string"},
	}
}

type ReplaceArgs struct {
	Path      string `json:"path"`
	OldString string `json:"old_string"`
	NewString string `json:"new_string"`
}

func (t *ReplaceTool) Execute(args string) (string, error) {
	var toolArgs ReplaceArgs
	if err := json.Unmarshal([]byte(args), &toolArgs); err != nil {
		return "", fmt.Errorf("invalid arguments for replace: %w", err)
	}

	if toolArgs.Path == "" || toolArgs.OldString == "" || toolArgs.NewString == "" {
		return "", fmt.Errorf("path, old_string, and new_string arguments are required for replace")
	}

	// Read the file content
	contentBytes, err := os.ReadFile(toolArgs.Path)
	if err != nil {
		return "", fmt.Errorf("error reading file '%s': %w", toolArgs.Path, err)
	}
	content := string(contentBytes)

	// Find and replace the first occurrence
	index := strings.Index(content, toolArgs.OldString)
	if index == -1 {
		return "", fmt.Errorf("old_string not found in file '%s'", toolArgs.Path)
	}

	modifiedContent := content[:index] + toolArgs.NewString + content[index+len(toolArgs.OldString):]

	// Write the modified content back to the file
	err = os.WriteFile(toolArgs.Path, []byte(modifiedContent), 0644)
	if err != nil {
		return "", fmt.Errorf("error writing to file '%s': %w", toolArgs.Path, err)
	}

	return fmt.Sprintf("Successfully replaced first occurrence of string in %s", toolArgs.Path), nil
}
