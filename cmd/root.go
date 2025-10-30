package cmd

import (
	"fmt"
	"os"
	"strings"

	"tachigoma/internal/llm"
	"tachigoma/internal/tui"

	"github.com/charmbracelet/bubbletea"
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

		if prompt != "" {
			promptProvided = true
			currentPrompt = prompt
		} else if len(args) > 0 {
			promptProvided = true
			currentPrompt = strings.Join(args, " ")
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

// directAPICall handles the one-off command mode.
func directAPICall(p string) {
	apiKey := viper.GetString("api_key")
	apiURL := viper.GetString("api_url")
	model := viper.GetString("model")

	if apiKey == "" {
		fmt.Println("API key is not set. Please configure it in .tachigoma.yaml or environment variables.")
		os.Exit(1)
	}

	client := llm.NewClient(apiURL, apiKey)

	fmt.Println("You:", p)
	fmt.Print("Tachigoma: ...")

	messages := []llm.Message{
		{Role: "user", Content: p},
	}

	response, err := client.Completion(messages, model)
	if err != nil {
		fmt.Fprintf(os.Stderr, "\nError calling LLM API: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("\rTachigoma: %s  \n", response)
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
	viper.SetConfigName(".tachigoma")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	viper.AddConfigPath("$HOME")

	viper.SetDefault("api_url", "http://localhost:3000/v1")
	viper.SetDefault("model", "gpt-3.5-turbo")

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			// Config file not found
		} else {
			fmt.Fprintf(os.Stderr, "Error reading config file: %s\n", err)
			os.Exit(1)
		}
	}
}
