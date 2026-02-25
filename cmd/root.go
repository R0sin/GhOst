package cmd

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"tachigoma/internal/llm"
	"tachigoma/internal/tui"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	prompt string
)

var rootCmd = &cobra.Command{
	Use:   "tachigoma",
	Short: "Tachigoma is a CLI client for LLM.",
	Long:  `A simple and powerful CLI client for interacting with Large Language Models.`,
	Run: func(cmd *cobra.Command, args []string) {
		var promptProvided bool
		var currentPrompt string

		// Read prompt from -p flag or positional args
		if prompt != "" {
			promptProvided = true
			currentPrompt = prompt
		} else if len(args) > 0 {
			promptProvided = true
			currentPrompt = strings.Join(args, " ")
		}

		// Check for pipe input (e.g., cat file.txt | tachigoma -p "explain")
		pipeContent := readPipeInput()
		if pipeContent != "" {
			promptProvided = true
			if currentPrompt != "" {
				// Combine pipe content with user prompt
				currentPrompt = fmt.Sprintf("以下是输入内容:\n```\n%s\n```\n\n%s", pipeContent, currentPrompt)
			} else {
				// Only pipe content, no additional prompt
				currentPrompt = pipeContent
			}
		}

		if promptProvided {
			// If a prompt is given, perform a direct API call and exit.
			directAPICall(currentPrompt)
		} else {
			// If no prompt is given, launch the interactive TUI.
			callTUI()
		}
	},
}

// isInputFromPipe checks if stdin is receiving input from a pipe or redirect.
func isInputFromPipe() bool {
	stat, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return (stat.Mode() & os.ModeCharDevice) == 0
}

// readPipeInput reads all content from stdin if it's a pipe.
// Returns empty string if stdin is a terminal (interactive mode).
func readPipeInput() string {
	if !isInputFromPipe() {
		return ""
	}

	reader := bufio.NewReader(os.Stdin)
	content, err := io.ReadAll(reader)
	if err != nil {
		return ""
	}

	return strings.TrimSpace(string(content))
}

// directAPICall handles the one-off command mode with streaming output.
func directAPICall(p string) {
	apiKey := viper.GetString("api_key")
	apiURL := viper.GetString("api_url")
	model := viper.GetString("model")

	if apiKey == "" {
		fmt.Println("API key is not set. Please configure it in .tachigoma.yaml or environment variables.")
		os.Exit(1)
	}

	client := llm.NewClient(apiURL, apiKey)

	messages := []llm.Message{
		{Role: "user", Content: p},
	}

	ctx := context.Background()
	streamCh := client.CompletionStream(ctx, messages, model, nil) // No tools in direct mode

	// Consume the stream and print to stdout in real-time
	for event := range streamCh {
		switch event.Type {
		case llm.EventStreamContent:
			fmt.Print(event.Content)
		case llm.EventError:
			fmt.Fprintf(os.Stderr, "\nError: %v\n", event.Error)
			os.Exit(1)
		case llm.EventToolCall:
			// In direct mode, we don't handle tool calls
			fmt.Println("\n[Tool call requested. Please use interactive mode for tool usage.]")
		case llm.EventStreamEnd:
			fmt.Println() // Final newline
		}
	}
}

// callTUI handles the interactive session mode.
func callTUI() {
	// We need to create the client and pass it to the TUI
	apiKey := viper.GetString("api_key")
	apiURL := viper.GetString("api_url")
	model := viper.GetString("model")

	if apiKey == "" {
		fmt.Println("API key is not set. Please configure it in .tachigoma.yaml or environment variables.")
		os.Exit(1)
	}

	client := llm.NewClient(apiURL, apiKey)

	initialModel := tui.NewModel(client, model) // Pass client and model to TUI
	program := tea.NewProgram(initialModel)

	if _, err := program.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error running program: %v\n", err)
		os.Exit(1)
	}
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)
	rootCmd.PersistentFlags().StringVarP(&prompt, "prompt", "p", "", "Prompt for a one-off question. If empty, starts interactive TUI mode.")
}

func initConfig() {
	// 配置文件名和类型
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")

	// 配置搜索路径（优先级从高到低）：
	// 1. XDG 用户配置目录 (Linux/macOS: ~/.config/tachigoma, Windows: %APPDATA%\tachigoma)
	if xdgConfig := os.Getenv("XDG_CONFIG_HOME"); xdgConfig != "" {
		viper.AddConfigPath(filepath.Join(xdgConfig, "tachigoma"))
	} else {
		home, err := os.UserHomeDir()
		if err == nil {
			if runtime.GOOS == "windows" {
				// Windows: 使用 %APPDATA%
				if appData := os.Getenv("APPDATA"); appData != "" {
					viper.AddConfigPath(filepath.Join(appData, "tachigoma"))
				}
			} else {
				// Linux/macOS: 使用 ~/.config/tachigoma
				viper.AddConfigPath(filepath.Join(home, ".config", "tachigoma"))
			}
		}
	}

	// 2. 用户主目录 (向后兼容 .tachigoma.yaml)
	home, err := os.UserHomeDir()
	if err == nil {
		viper.AddConfigPath(home)
	}

	// 3. 系统级配置目录 (仅 Linux/macOS)
	if runtime.GOOS != "windows" {
		viper.AddConfigPath("/etc/tachigoma")
	}

	// 设置环境变量前缀，例如 TACHIGOMA_API_KEY
	viper.SetEnvPrefix("TACHIGOMA")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()

	// 设置默认值
	viper.SetDefault("api_url", "http://localhost:3000/v1")
	viper.SetDefault("model", "gpt-3.5-turbo")

	// 尝试读取配置文件
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			// 配置文件未找到 - 尝试旧的配置文件名 .tachigoma.yaml
			viper.SetConfigName(".tachigoma")
			if err := viper.ReadInConfig(); err != nil {
				// 两个配置文件都未找到，依赖环境变量和默认值
			}
		} else {
			fmt.Fprintf(os.Stderr, "Error reading config file: %s\n", err)
			os.Exit(1)
		}
	}
}
