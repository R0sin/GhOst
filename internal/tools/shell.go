package tools

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)

// RunShellCommandTool defines the tool for executing shell commands.
type RunShellCommandTool struct{}

// RunShellCommandArgs defines the arguments for the RunShellCommandTool.
type RunShellCommandArgs struct {
	Command   string `json:"command"`
	Directory string `json:"directory,omitempty"` // Optional directory to run the command in
}

func (t *RunShellCommandTool) Name() string {
	return "run_shell_command"
}

func (t *RunShellCommandTool) Description() string {
	return `Executes a shell command on the user's operating system and returns the combined output from stdout and stderr. 
This tool is powerful and can modify system state. 
Usage: {"command": "<command_to_run>", "directory": "<optional_path>"}`
}

func (t *RunShellCommandTool) Parameters() any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"command": map[string]any{
				"type":        "string",
				"description": "The shell command to execute.",
			},
			"directory": map[string]any{
				"type":        "string",
				"description": "Optional: The working directory where the command should be executed. If not provided, it uses the current directory of the application.",
			},
		},
		"required": []string{"command"},
	}
}

// RequiresConfirmation makes this a "dangerous" tool that needs user approval.
func (t *RunShellCommandTool) RequiresConfirmation() bool {
	return true
}

// Execute runs the shell command.
func (t *RunShellCommandTool) Execute(args string) (string, error) {
	var toolArgs RunShellCommandArgs
	if err := json.Unmarshal([]byte(args), &toolArgs); err != nil {
		return "", fmt.Errorf("invalid arguments for run_shell_command: %w. Expected JSON: {\"command\": \"...\"}", err)
	}

	if strings.TrimSpace(toolArgs.Command) == "" {
		return "", fmt.Errorf("command argument cannot be empty")
	}

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		// Windows 系统
		cmd = exec.Command("cmd", "/C", toolArgs.Command)
	} else {
		// Linux, macOS, and other Unix-like systems
		cmd = exec.Command("sh", "-c", toolArgs.Command)
	}

	// Set the working directory if provided.
	if toolArgs.Directory != "" {
		cmd.Dir = toolArgs.Directory
	}

	// Use CombinedOutput to get both stdout and stderr in one slice.
	output, err := cmd.CombinedOutput()

	if err != nil {
		// If there was an error (e.g., non-zero exit code), we still want to return the output,
		// as it often contains the error message from the command itself.
		return "", fmt.Errorf("command failed with exit code: %v\nOutput:\n%s", err, string(output))
	}

	return string(output), nil
}
