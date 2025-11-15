package llm

import (
	_ "embed"
	"fmt"
	"tachigoma/internal/tools"

	"github.com/charmbracelet/bubbletea"
)

//go:embed prompt.md
var systemPromptContent string

// Agent is the core logic unit of the application. It is UI-independent.
type Agent struct {
	client       *Client
	modelName    string
	toolRegistry map[string]tools.Tool

	// State
	messages           []Message
	pendingToolCalls   []ToolCall
	confirmingToolCall ToolCall
	isConfirming       bool

	// Live state for streaming
	lastStreamedContent string
}

// NewAgent creates a new agent.
func NewAgent(client *Client, modelName string) *Agent {
	// Initialize and register all available tools.
	availableTools := []tools.Tool{
		&tools.ListDirectoryTool{},
		&tools.ReadFileTool{},
		&tools.WriteFileTool{},
		&tools.SearchFileContentTool{},
		&tools.GlobTool{},
		&tools.ReplaceTool{},
		&tools.RunShellCommandTool{},
	}

	toolRegistry := make(map[string]tools.Tool)
	for _, tool := range availableTools {
		toolRegistry[tool.Name()] = tool
	}

	return &Agent{
		client:       client,
		modelName:    modelName,
		toolRegistry: toolRegistry,
		messages: []Message{
			{Role: "system", Content: systemPromptContent},
		},
	}
}

// ViewState is a snapshot of the agent's state, intended for rendering by the UI.
type ViewState struct {
	Messages            []Message
	LastStreamedContent string
	IsConfirming        bool
	ConfirmingToolCall  ToolCall
}

// GetViewState returns a snapshot of the current state for rendering.
func (a *Agent) GetViewState() ViewState {
	return ViewState{
		Messages:            a.messages,
		LastStreamedContent: a.lastStreamedContent,
		IsConfirming:        a.isConfirming,
		ConfirmingToolCall:  a.confirmingToolCall,
	}
}

// getAvailableToolsAsJSON converts the registered tools into the JSON format expected by the API.
func (a *Agent) getAvailableToolsAsJSON() []Tool {
	var availableTools []Tool
	for _, tool := range a.toolRegistry {
		availableTools = append(availableTools, Tool{
			Type: "function",
			Function: struct {
				Name        string `json:"name"`
				Description string `json:"description"`
				Parameters  any    `json:"parameters"`
			}{
				Name:        tool.Name(),
				Description: tool.Description(),
				Parameters:  tool.Parameters(),
			},
		})
	}
	return availableTools
}

// HandleUserInput starts a new conversation turn.
func (a *Agent) HandleUserInput(input string) tea.Cmd {
	a.messages = append(a.messages, Message{Role: "user", Content: input})
	return a.client.CompletionStream(a.messages, a.modelName, a.getAvailableToolsAsJSON())
}

// HandleStreamStart prepares the agent for a new stream of messages.
func (a *Agent) HandleStreamStart() {
	a.lastStreamedContent = ""
	a.messages = append(a.messages, Message{Role: "assistant", Content: ""})
}

// HandleStreamContent appends content to the last message.
func (a *Agent) HandleStreamContent(content string) {
	if len(a.messages) > 0 {
		last := len(a.messages) - 1
		a.messages[last].Content += content
		a.lastStreamedContent = a.messages[last].Content
	}
}

// HandleToolCallRequest sets up the agent to process tool calls.
func (a *Agent) HandleToolCallRequest(msg AssistantToolCallMsg) tea.Cmd {
	// 如果最后一条消息是 assistant 消息（在流式输出过程中创建的），
	// 我们应该将 ToolCalls 添加到这条消息中，而不是创建新消息
	if len(a.messages) > 0 && a.messages[len(a.messages)-1].Role == "assistant" {
		// 更新现有的 assistant 消息，添加 ToolCalls
		a.messages[len(a.messages)-1].ToolCalls = msg.Message.ToolCalls
	} else {
		// 否则，添加新的 assistant 消息
		a.messages = append(a.messages, msg.Message)
	}
	a.pendingToolCalls = msg.Message.ToolCalls
	a.lastStreamedContent = ""
	return a.processToolCalls()
}

// HandleToolResult adds a tool result to the message history and continues processing.
func (a *Agent) HandleToolResult(toolCallID, result string) tea.Cmd {
	a.messages = append(a.messages, Message{
		Role:       "tool",
		ToolCallID: toolCallID,
		Content:    result,
	})
	return a.processToolCalls()
}

// HandleConfirmation handles the user's decision on a tool call confirmation.
func (a *Agent) HandleConfirmation(confirmed bool) tea.Cmd {
	a.isConfirming = false
	toolCall := a.confirmingToolCall
	a.pendingToolCalls = a.pendingToolCalls[1:] // Consume the call

	if confirmed {
		return a.executeTool(toolCall)
	}

	// User denied, create a synthetic result and handle it.
	result := "User denied execution of tool: " + toolCall.Function.Name
	return a.HandleToolResult(toolCall.ID, result)
}

// --- Internal Logic ---

func (a *Agent) processToolCalls() tea.Cmd {
	if len(a.pendingToolCalls) == 0 {
		return a.client.CompletionStream(a.messages, a.modelName, a.getAvailableToolsAsJSON())
	}

	toolCall := a.pendingToolCalls[0]
	tool, ok := a.toolRegistry[toolCall.Function.Name]
	if !ok {
		return func() tea.Msg {
			return ErrorMsg{Err: fmt.Errorf("tool %s not found in registry", toolCall.Function.Name)}
		}
	}

	if tool.RequiresConfirmation() {
		a.confirmingToolCall = toolCall
		a.isConfirming = true
		// 返回一个命令来通知 UI 需要确认，而不是返回 nil
		return func() tea.Msg {
			return ConfirmationRequiredMsg{ToolCall: toolCall}
		}
	}

	a.pendingToolCalls = a.pendingToolCalls[1:]
	return a.executeTool(toolCall)
}

func (a *Agent) executeTool(toolCall ToolCall) tea.Cmd {
	return func() tea.Msg {
		tool, _ := a.toolRegistry[toolCall.Function.Name]
		result, err := tool.Execute(toolCall.Function.Arguments)
		if err != nil {
			result = fmt.Sprintf("Error executing tool %s: %v", toolCall.Function.Name, err)
		}

		return ToolResultMsg{
			ToolCallID: toolCall.ID,
			Result:     result,
		}
	}
}
