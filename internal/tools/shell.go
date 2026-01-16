package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"golang.org/x/text/encoding/simplifiedchinese"
)

// DefaultCommandTimeout is the default timeout for shell commands.
const DefaultCommandTimeout = 60 * time.Second

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

// Execute runs the shell command with a timeout.
func (t *RunShellCommandTool) Execute(args string) (string, error) {
	var toolArgs RunShellCommandArgs
	if err := json.Unmarshal([]byte(args), &toolArgs); err != nil {
		return "", fmt.Errorf("invalid arguments for run_shell_command: %w. Expected JSON: {\"command\": \"...\"}", err)
	}

	if strings.TrimSpace(toolArgs.Command) == "" {
		return "", fmt.Errorf("command argument cannot be empty")
	}

	// Create context with timeout to prevent commands from running indefinitely
	ctx, cancel := context.WithTimeout(context.Background(), DefaultCommandTimeout)
	defer cancel()

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		// Windows 系统
		cmd = exec.CommandContext(ctx, "cmd", "/C", toolArgs.Command)
	} else {
		// Linux, macOS, and other Unix-like systems
		cmd = exec.CommandContext(ctx, "sh", "-c", toolArgs.Command)
	}

	// Set the working directory if provided.
	if toolArgs.Directory != "" {
		cmd.Dir = toolArgs.Directory
	}

	// Use CombinedOutput to get both stdout and stderr in one slice.
	output, err := cmd.CombinedOutput()

	// On Windows, convert GBK output to UTF-8
	outputStr := string(output)
	if runtime.GOOS == "windows" {
		if decoded, decErr := simplifiedchinese.GBK.NewDecoder().Bytes(output); decErr == nil {
			outputStr = string(decoded)
		}
	}

	if err != nil {
		// Check if the error was due to timeout
		if ctx.Err() == context.DeadlineExceeded {
			return "", fmt.Errorf("command timed out after %v\nPartial output:\n%s", DefaultCommandTimeout, outputStr)
		}
		// If there was an error (e.g., non-zero exit code), we still want to return the output,
		// as it often contains the error message from the command itself.
		return "", fmt.Errorf("command failed with exit code: %v\nOutput:\n%s", err, outputStr)
	}

	return outputStr, nil
}
