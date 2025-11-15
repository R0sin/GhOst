package llm

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/charmbracelet/bubbletea"
)

// Client is the API client for the LLM.
type Client struct {
	apiURL string
	apiKey string
	http   *http.Client
}

// NewClient creates a new LLM client.
func NewClient(apiURL, apiKey string) *Client {
	return &Client{
		apiURL: apiURL,
		apiKey: apiKey,
		http:   &http.Client{},
	}
}

// Completion sends a list of messages to the LLM and returns the response.
func (c *Client) Completion(messages []Message, model string) (string, error) {
	// For this non-streaming mode, we won't send tools, just a simple chat.
	reqBody := CompletionRequest{
		Model:    model,
		Messages: messages,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("error marshalling request body: %w", err)
	}

	req, err := http.NewRequest("POST", c.apiURL+"/chat/completions", bytes.NewBuffer(jsonBody))
	if err != nil {
		return "", fmt.Errorf("error creating request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("error making request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var compResp CompletionResponse
	if err := json.NewDecoder(resp.Body).Decode(&compResp); err != nil {
		return "", fmt.Errorf("error decoding response: %w", err)
	}

	if len(compResp.Choices) > 0 && compResp.Choices[0].Message.Content != "" {
		return compResp.Choices[0].Message.Content, nil
	}

	// Handle the case where the model wants to call a tool, even in non-streaming mode.
	// For this simple mode, we'll just indicate that a tool call was attempted.
	if len(compResp.Choices) > 0 && len(compResp.Choices[0].Message.ToolCalls) > 0 {
		return "[Tachigoma wanted to use a tool. Please use interactive mode to allow tool usage.]", nil
	}

	return "", fmt.Errorf("no response choices found")
}

// --- Client Methods ---
// CompletionStream sends a list of messages and returns a command that streams the response.
func (c *Client) CompletionStream(messages []Message, model string, tools []Tool) tea.Cmd {
	return func() tea.Msg {
		ch := make(chan tea.Msg)

		go func() {
			defer close(ch)
			c.runCompletionStream(messages, model, tools, ch)
		}()

		return Stream(ch)
	}
}

// runCompletionStream handles the actual logic of streaming, tool calls, and looping.
func (c *Client) runCompletionStream(messages []Message, model string, tools []Tool, ch chan tea.Msg) {
	reqBody := CompletionRequest{
		Model:    model,
		Messages: messages,
		Stream:   true,
		Tools:    tools,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		ch <- ErrorMsg{fmt.Errorf("error marshalling request body: %w", err)}
		return
	}

	req, err := http.NewRequest("POST", c.apiURL+"/chat/completions", bytes.NewBuffer(jsonBody))
	if err != nil {
		ch <- ErrorMsg{fmt.Errorf("error creating request: %w", err)}
		return
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Cache-Control", "no-cache")
	req.Header.Set("Connection", "keep-alive")

	resp, err := c.http.Do(req)
	if err != nil {
		ch <- ErrorMsg{fmt.Errorf("error making request: %w", err)}
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		ch <- ErrorMsg{fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(bodyBytes))}
		return
	}

	ch <- StreamStartMsg{}

	// Variables to aggregate the response
	var toolCalls []ToolCall

	reader := bufio.NewReader(resp.Body)
	for {
		line, err := reader.ReadBytes('\n')
		if err != nil {
			if err != io.EOF {
				ch <- ErrorMsg{fmt.Errorf("error reading stream: %w", err)}
			}
			break // End of stream
		}

		lineStr := string(line)
		if !strings.HasPrefix(lineStr, "data: ") {
			continue
		}

		data := strings.TrimPrefix(lineStr, "data: ")
		data = strings.TrimSpace(data)

		if data == "[DONE]" {
			break
		}

		var streamResp StreamCompletionResponse
		if err := json.Unmarshal([]byte(data), &streamResp); err != nil {
			continue
		}

		if len(streamResp.Choices) > 0 {
			choice := streamResp.Choices[0]

			// Aggregate content
			if choice.Delta.Content != "" {
				ch <- StreamContentMsg{Content: choice.Delta.Content}
			}

			// Aggregate tool calls
			if len(choice.Delta.ToolCalls) > 0 {
				for _, toolCallDelta := range choice.Delta.ToolCalls {
					if len(toolCalls) <= toolCallDelta.Index {
						// Expand the slice if a new tool call index appears
						toolCalls = append(toolCalls, make([]ToolCall, toolCallDelta.Index-len(toolCalls)+1)...)
					}
					call := &toolCalls[toolCallDelta.Index]
					if toolCallDelta.ID != "" {
						call.ID = toolCallDelta.ID
					}
					if toolCallDelta.Type != "" {
						call.Type = toolCallDelta.Type
					}
					call.Function.Name += toolCallDelta.Function.Name
					call.Function.Arguments += toolCallDelta.Function.Arguments
				}
			}
		}
	}

	// After stream, check for tool calls
	if len(toolCalls) > 0 {
		// Create the assistant's message with the tool call requests.
		assistantMessage := Message{
			Role:      "assistant",
			ToolCalls: toolCalls,
		}

		// Send this message to the TUI. The TUI will handle execution,
		// user confirmation, and continuing the conversation.
		ch <- AssistantToolCallMsg{Message: assistantMessage}

		// The rest of the stream processing for this turn is now complete.
		// The TUI will initiate the next turn.
	}

	ch <- StreamEndMsg{}
}
