package llm

import (
	"GhOst/internal/tools"
	"fmt"
	"github.com/charmbracelet/bubbletea"
)

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
	// Initialize the tool registry and register tools.
	toolRegistry := make(map[string]tools.Tool)

	listDirTool := &tools.ListDirectoryTool{}
	toolRegistry[listDirTool.Name()] = listDirTool

	readFileTool := &tools.ReadFileTool{}
	toolRegistry[readFileTool.Name()] = readFileTool

	writeFileTool := &tools.WriteFileTool{}
	toolRegistry[writeFileTool.Name()] = writeFileTool

	searchFileContentTool := &tools.SearchFileContentTool{}
	toolRegistry[searchFileContentTool.Name()] = searchFileContentTool

	globTool := &tools.GlobTool{}
	toolRegistry[globTool.Name()] = globTool

	replaceTool := &tools.ReplaceTool{}
	toolRegistry[replaceTool.Name()] = replaceTool

	return &Agent{
		client:       client,
		modelName:    modelName,
		toolRegistry: toolRegistry,
		messages:     []Message{},
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
	a.messages = append(a.messages, msg.Message)
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
		return nil
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
