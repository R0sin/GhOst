package llm

import "github.com/charmbracelet/bubbletea"

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

// --- TUI Message Types ---

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

// AssistantToolCallMsg is sent when the model requests tool calls.
type AssistantToolCallMsg struct {
	Message Message
}

// ErrorMsg is sent when an error occurs.
type ErrorMsg struct{ Err error }

// ToolResultMsg is sent when a tool has finished executing.
type ToolResultMsg struct {
	ToolCallID string
	Result     string
}

// ConfirmationRequiredMsg is sent when a tool requires user confirmation.
type ConfirmationRequiredMsg struct {
	ToolCall ToolCall
}
