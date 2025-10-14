package llm

import (
	"GhOst/internal/tools"
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/charmbracelet/bubbletea"
	"io"
	"log"
	"net/http"
	"strings"
)

// Client is the API client for the LLM.
type Client struct {
	apiURL       string
	apiKey       string
	http         *http.Client
	toolRegistry map[string]tools.Tool
}

// NewClient creates a new LLM client.
func NewClient(apiURL, apiKey string) *Client {
	// Initialize the tool registry and register tools.
	toolRegistry := make(map[string]tools.Tool)

	listDirTool := &tools.ListDirectoryTool{}
	toolRegistry[listDirTool.Name()] = listDirTool

	readFileTool := &tools.ReadFileTool{}
	toolRegistry[readFileTool.Name()] = readFileTool

	return &Client{
		apiURL:       apiURL,
		apiKey:       apiKey,
		http:         &http.Client{},
		toolRegistry: toolRegistry,
	}
}

// --- API Data Structures ---

// Message is a single message in a chat completion request.
type Message struct {
	Role       string     `json:"role"`
	Content    string     `json:"content,omitempty"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
}

// ToolCall represents a complete tool call.
type ToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
}

// ToolCallDelta represents a chunk of a tool call from the stream.
type ToolCallDelta struct {
	Index    int    `json:"index"`
	ID       string `json:"id,omitempty"`
	Type     string `json:"type,omitempty"`
	Function struct {
		Name      string `json:"name,omitempty"`
		Arguments string `json:"arguments,omitempty"`
	} `json:"function,omitempty"`
}

// Tool represents the definition of a tool that can be called.
type Tool struct {
	Type     string `json:"type"`
	Function struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Parameters  any    `json:"parameters"`
	} `json:"function"`
}

// CompletionRequest is the request body for a chat completion.
type CompletionRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
	Stream   bool      `json:"stream,omitempty"`
	Tools    []Tool    `json:"tools,omitempty"`
}

// CompletionResponse is the response body for a non-streaming chat completion.
type CompletionResponse struct {
	Choices []struct {
		Message Message `json:"message"`
	} `json:"choices"`
}

// StreamChoice is a single choice in a streaming chat completion response.
type StreamChoice struct {
	Index int `json:"index"`
	Delta struct {
		Content   string          `json:"content"`
		ToolCalls []ToolCallDelta `json:"tool_calls"`
	} `json:"delta"`
	FinishReason string `json:"finish_reason"`
}

// StreamCompletionResponse is the response body for a streaming chat completion.
type StreamCompletionResponse struct {
	Choices []StreamChoice `json:"choices"`
}

// --- Client Methods ---

// getAvailableToolsAsJSON converts the registered tools into the JSON format expected by the API.
func (c *Client) getAvailableToolsAsJSON() []Tool {
	var availableTools []Tool
	for _, tool := range c.toolRegistry {
		availableTools = append(availableTools, Tool{
			Type: "function",
			Function: struct {
				Name        string `json:"name"`
				Description string `json:"description"`
				Parameters  any    `json:"parameters"`
			}{
				Name:        tool.Name(),
				Description: tool.Description(),
				Parameters:  tool.Parameters(), // Call the tool's own method
			},
		})
	}
	return availableTools
}

// Completion sends a list of messages to the LLM and returns the response.
func (c *Client) Completion(messages []Message, model string) (string, error) {
	// This function is now less important, as the primary mode will be streaming.
	// It could be updated to support tool calls in a non-streaming way if needed.
	// For now, we leave it as is.
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

	return "", fmt.Errorf("no response choices found")
}

// Stream is a channel of messages from the LLM stream.
type Stream chan tea.Msg

// StreamStartMsg is sent when the stream starts.
type StreamStartMsg struct{}

// StreamContentMsg is sent for each content chunk.
type StreamContentMsg struct {
	Content string
}

// StreamEndMsg is sent when the stream ends.
type StreamEndMsg struct{}

// ErrorMsg is sent when an error occurs.
type ErrorMsg struct{ Err error }

// CompletionStream sends a list of messages and returns a command that streams the response.
func (c *Client) CompletionStream(messages []Message, model string) tea.Cmd {
	return func() tea.Msg {
		ch := make(chan tea.Msg)

		go func() {
			defer close(ch)
			c.runCompletionStream(messages, model, ch)
		}()

		return Stream(ch)
	}
}

// runCompletionStream handles the actual logic of streaming, tool calls, and looping.
func (c *Client) runCompletionStream(messages []Message, model string, ch chan tea.Msg) {
	reqBody := CompletionRequest{
		Model:    model,
		Messages: messages,
		Stream:   true,
		Tools:    c.getAvailableToolsAsJSON(),
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
	finishReason := ""

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
			finishReason = choice.FinishReason

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
	if finishReason == "tool_calls" {

		// Append the assistant's message with tool call requests to history
		assistantMessage := Message{
			Role:      "assistant",
			ToolCalls: toolCalls,
		}
		newMessages := append(messages, assistantMessage)

		// Execute tools and append results
		for _, toolCall := range toolCalls {
			tool, ok := c.toolRegistry[toolCall.Function.Name]
			if !ok {
				log.Printf("Error: tool '%s' not found", toolCall.Function.Name)
				continue
			}

			result, err := tool.Execute(toolCall.Function.Arguments)
			if err != nil {
				log.Printf("Error executing tool '%s': %v", toolCall.Function.Name, err)
				result = fmt.Sprintf("Error: %v", err)
			}

			// Append tool result to messages
			newMessages = append(newMessages, Message{
				Role:       "tool",
				ToolCallID: toolCall.ID,
				Content:    result,
			})
		}

		// Recurse with the new messages to get the final response
		c.runCompletionStream(newMessages, model, ch)
		return
	}

	ch <- StreamEndMsg{}
}
