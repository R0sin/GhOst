package llm

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/charmbracelet/bubbletea"
	"io"
	"net/http"
	"strings"
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

// Message is a single message in a chat completion request.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// CompletionRequest is the request body for a non-streaming chat completion.
type CompletionRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
	Stream   bool      `json:"stream,omitempty"`
}

// CompletionResponse is the response body for a non-streaming chat completion.
type CompletionResponse struct {
	Choices []struct {
		Message Message `json:"message"`
	} `json:"choices"`
}

// StreamChoice is a single choice in a streaming chat completion response.
type StreamChoice struct {
	Delta struct {
		Content string `json:"content"`
	} `json:"delta"`
	FinishReason string `json:"finish_reason"`
}

// StreamCompletionResponse is the response body for a streaming chat completion.
type StreamCompletionResponse struct {
	Choices []StreamChoice `json:"choices"`
}

// Completion sends a list of messages to the LLM and returns the response.
func (c *Client) Completion(messages []Message, model string) (string, error) {
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
		reqBody := CompletionRequest{
			Model:    model,
			Messages: messages,
			Stream:   true,
		}

		jsonBody, err := json.Marshal(reqBody)
		if err != nil {
			return ErrorMsg{fmt.Errorf("error marshalling request body: %w", err)}
		}

		req, err := http.NewRequest("POST", c.apiURL+"/chat/completions", bytes.NewBuffer(jsonBody))
		if err != nil {
			return ErrorMsg{fmt.Errorf("error creating request: %w", err)}
		}

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
		req.Header.Set("Accept", "text/event-stream")
		req.Header.Set("Cache-Control", "no-cache")
		req.Header.Set("Connection", "keep-alive")

		resp, err := c.http.Do(req)
		if err != nil {
			return ErrorMsg{fmt.Errorf("error making request: %w", err)}
		}

		ch := make(chan tea.Msg)

		go func() {
			defer close(ch)
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				bodyBytes, _ := io.ReadAll(resp.Body)
				ch <- ErrorMsg{fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(bodyBytes))}
				return
			}

			ch <- StreamStartMsg{}

			reader := bufio.NewReader(resp.Body)
			for {
				line, err := reader.ReadBytes('\n')
				if err != nil {
					if err != io.EOF {
						ch <- ErrorMsg{fmt.Errorf("error reading stream: %w", err)}
					}
					ch <- StreamEndMsg{}
					return
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
					if choice.FinishReason == "stop" {
						break
					}
					ch <- StreamContentMsg{Content: choice.Delta.Content}
				}
			}
			ch <- StreamEndMsg{}
		}()

		return Stream(ch)
	}
}
