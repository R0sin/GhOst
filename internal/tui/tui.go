package tui

import (
	"fmt"
	"strings"
	"tachigoma/internal/llm"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
)

// --- Styles ---
var (
	helpStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))

	// Message role styles
	userRoleStyle      = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("70"))
	assistantRoleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("66"))

	// Tool call styles
	toolCallStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("214")) // Orange
	toolArgStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("243")) // Gray
	resultLabel    = lipgloss.NewStyle().Foreground(lipgloss.Color("114")) // Light green
	resultContent  = lipgloss.NewStyle().Foreground(lipgloss.Color("248")) // Light gray
	truncatedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("243")).Italic(true)

	// Tool box style
	toolBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("240")).
			Padding(0, 1).
			MarginLeft(2)

	// Error style
	errorStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
)

// --- Conversation Block Types ---

// ToolCallWithResult pairs a tool call with its result.
type ToolCallWithResult struct {
	Call   llm.ToolCall
	Result string
}

// AssistantSegment represents a segment of assistant response (content + optional tool calls).
type AssistantSegment struct {
	Content   string               // Text content before tool calls
	ToolCalls []ToolCallWithResult // Tool calls in this segment
}

// ConversationBlock represents a renderable unit in the conversation.
type ConversationBlock struct {
	Type      string             // "user" or "assistant"
	Content   string             // Text content (for user messages)
	Segments  []AssistantSegment // Ordered segments (for assistant messages)
	IsLastMsg bool               // Whether this is the last message
	MsgIndex  int                // Original message index
}

// model is the state of our TUI application.
type model struct {
	viewport        viewport.Model
	textarea        textarea.Model
	agent           *llm.Agent             // The new core logic handler
	streamCh        <-chan llm.StreamEvent // Channel for receiving streaming events
	loading         bool
	lastContent     string // Stores the live content of the current streaming message
	err             error
	availableHeight int                   // Available height for the viewport
	ready           bool                  // Whether the UI has been sized and is ready for rendering
	renderer        *glamour.TermRenderer // Cached markdown renderer for performance
}

// --- TUI Messages ---

// waitForStreamEvent waits for the next event from the stream channel and converts it to a tea.Msg.
func waitForStreamEvent(ch <-chan llm.StreamEvent) tea.Cmd {
	return func() tea.Msg {
		event, ok := <-ch
		if !ok {
			// Channel closed, stream ended
			return llm.StreamEndMsg{}
		}

		// Convert StreamEvent to appropriate tea.Msg
		switch event.Type {
		case llm.EventStreamStart:
			return llm.StreamStartMsg{}
		case llm.EventStreamContent:
			return llm.StreamContentMsg{Content: event.Content}
		case llm.EventStreamEnd:
			return llm.StreamEndMsg{}
		case llm.EventToolCall:
			return llm.AssistantToolCallMsg{
				Message: llm.Message{
					Role:      "assistant",
					ToolCalls: event.ToolCalls,
				},
			}
		case llm.EventError:
			return llm.ErrorMsg{Err: event.Error}
		default:
			return nil
		}
	}
}

// safeGotoBottom scrolls to bottom only if the viewport is ready.
func (m *model) safeGotoBottom() {
	if m.ready && m.viewport.Height > 0 {
		m.viewport.GotoBottom()
	}
}

// updateViewportHeight adjusts the viewport height based on confirmation state.
func (m *model) updateViewportHeight() {
	viewState := m.agent.GetViewState()
	if viewState.IsConfirming {
		// Create a temporary confirmation box to measure its height
		confirmStyle := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("205")).
			Padding(1, 2)

		question := fmt.Sprintf(
			"Tachigoma wants to run the tool: %s\n\nArguments:\n%s\n\nDo you want to allow this?",
			viewState.ConfirmingToolCall.Function.Name,
			viewState.ConfirmingToolCall.Function.Arguments,
		)
		confirmationBox := confirmStyle.Render(question)
		confirmationBoxHeight := lipgloss.Height(confirmationBox)
		m.viewport.Height = m.availableHeight - confirmationBoxHeight
	} else {
		m.viewport.Height = m.availableHeight
	}
}

// toolResultMsg is sent when a tool has finished executing.
// It is defined in the llm package but handled here.

// --- TUI Commands ---

// NewModel creates the initial model for the TUI.
func NewModel(client *llm.Client, modelName string) tea.Model {
	ti := textarea.New()
	ti.Placeholder = "输入你的问题... (Enter 发送)"
	ti.Focus()

	vp := viewport.New(0, 0)

	// Create a cached renderer with dark style for consistent performance
	// Using fixed style instead of AutoStyle to avoid terminal detection overhead on SSH
	renderer, _ := glamour.NewTermRenderer(
		glamour.WithStylePath("dark"),
		glamour.WithWordWrap(0), // Disable word wrap, let viewport handle it
	)

	return model{
		agent:    llm.NewAgent(client, modelName),
		textarea: ti,
		viewport: vp,
		renderer: renderer,
	}
}

// Init is the first command that is run when the program starts.
func (m model) Init() tea.Cmd {
	return textarea.Blink
}

// Update handles incoming messages and updates the model accordingly.
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.availableHeight = msg.Height - m.textarea.Height() - lipgloss.Height(m.helpView())
		m.viewport.Height = m.availableHeight
		m.viewport.Width = msg.Width
		m.textarea.SetWidth(msg.Width)
		m.viewport.SetContent(m.renderConversation(true))
		m.ready = true // Mark UI as ready after first resize
		return m, nil

	// We've received the stream channel from Agent. Start listening for events.
	case llm.StreamChannelMsg:
		m.streamCh = msg.Channel
		return m, waitForStreamEvent(m.streamCh)

	case llm.StreamStartMsg:
		m.loading = true
		m.err = nil
		m.lastContent = ""
		m.agent.HandleStreamStart()
		return m, waitForStreamEvent(m.streamCh)

	case llm.StreamContentMsg:
		m.agent.HandleStreamContent(msg.Content)
		m.lastContent = m.agent.GetViewState().LastStreamedContent
		m.viewport.SetContent(m.renderConversation(false))
		m.safeGotoBottom()
		return m, waitForStreamEvent(m.streamCh)

	case llm.StreamEndMsg:
		m.loading = false
		m.streamCh = nil
		m.lastContent = ""
		m.viewport.SetContent(m.renderConversation(true))
		m.safeGotoBottom()
		return m, nil

	case llm.AssistantToolCallMsg:
		cmd = m.agent.HandleToolCallRequest(msg)
		m.updateViewportHeight() // Adjust height if confirmation dialog appears
		m.viewport.SetContent(m.renderConversation(true))
		m.safeGotoBottom()
		// 如果流订阅通道还存在，需要继续监听以接收 StreamEndMsg
		if m.streamCh != nil {
			return m, tea.Batch(cmd, waitForStreamEvent(m.streamCh))
		}
		return m, cmd

	case llm.ToolResultMsg:
		cmd = m.agent.HandleToolResult(msg.ToolCallID, msg.Result)
		m.updateViewportHeight() // Adjust height as confirmation state may change
		m.viewport.SetContent(m.renderConversation(true))
		m.safeGotoBottom()
		return m, cmd

	case llm.ConfirmationRequiredMsg:
		// 工具需要确认，更新视图以显示确认对话框
		m.updateViewportHeight()
		m.viewport.SetContent(m.renderConversation(true))
		m.safeGotoBottom()
		// 如果流订阅通道还存在，需要继续监听
		if m.streamCh != nil {
			return m, waitForStreamEvent(m.streamCh)
		}
		return m, nil

	case llm.ErrorMsg:
		m.loading = false
		m.err = msg.Err
		m.streamCh = nil
		m.viewport.SetContent(m.renderConversation(true))
		m.safeGotoBottom()
		return m, nil

	case tea.KeyMsg:
		viewState := m.agent.GetViewState()
		if viewState.IsConfirming {
			switch msg.String() {
			case "y", "Y":
				cmd = m.agent.HandleConfirmation(true)
				m.updateViewportHeight() // Restore height after confirmation
				return m, cmd
			case "n", "N":
				cmd = m.agent.HandleConfirmation(false)
				m.updateViewportHeight() // Restore height after denial
				return m, cmd
			}
		}

		switch msg.Type {
		case tea.KeyCtrlC:
			// If loading, interrupt the stream; otherwise quit
			if m.loading {
				m.agent.Cancel() // Cancel ongoing HTTP request
				m.loading = false
				m.streamCh = nil
				m.lastContent = ""
				m.err = fmt.Errorf("用户中断生成")
				m.viewport.SetContent(m.renderConversation(true))
				m.safeGotoBottom()
				return m, nil
			}
			return m, tea.Quit
		case tea.KeyCtrlD, tea.KeyEsc:
			// Always quit on Ctrl+D or Esc
			return m, tea.Quit
		case tea.KeyEnter:
			prompt := strings.TrimSpace(m.textarea.Value())
			if prompt != "" && !m.loading && !viewState.IsConfirming {
				cmd = m.agent.HandleUserInput(prompt)
				m.textarea.Reset()
				m.viewport.SetContent(m.renderConversation(true))
				m.safeGotoBottom()
				return m, cmd
			}
		}
	}

	// Pass messages to child components
	m.textarea, cmd = m.textarea.Update(msg)
	cmds = append(cmds, cmd)

	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

// View renders the UI based on the model's state.
func (m model) View() string {
	viewState := m.agent.GetViewState()
	var confirmationBox string

	if viewState.IsConfirming {
		confirmStyle := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("205")).
			Padding(1, 2)

		question := fmt.Sprintf(
			"Tachigoma wants to run the tool: %s\n\nArguments:\n%s\n\nDo you want to allow this?",
			viewState.ConfirmingToolCall.Function.Name,
			viewState.ConfirmingToolCall.Function.Arguments,
		)
		confirmationBox = confirmStyle.Render(question)
	}

	return lipgloss.JoinVertical(
		lipgloss.Left,
		confirmationBox, // Will be an empty string if not confirming
		m.viewport.View(),
		m.textarea.View(),
		m.helpView(),
	)
}

// helpView renders the help text at the bottom.
func (m model) helpView() string {
	if m.agent.GetViewState().IsConfirming {
		return helpStyle.Render("y: confirm | n: deny | esc/ctrl+d: quit")
	}
	if m.loading {
		return helpStyle.Render("ctrl+c: 中断生成 | esc/ctrl+d: quit")
	}
	return helpStyle.Render("enter: send | esc/ctrl+d: quit")
}

// --- Rendering Helper Functions ---

// truncateContent truncates content to maxRunes characters and maxLines lines.
// Returns the truncated content and whether truncation occurred.
func truncateContent(content string, maxRunes, maxLines int) (string, bool) {
	content = strings.TrimSpace(content)
	lines := strings.Split(content, "\n")
	truncated := false

	runes := []rune(content)
	if len(runes) > maxRunes {
		truncated = true
		content = string(runes[:maxRunes])
		lines = strings.Split(content, "\n")
	}

	if len(lines) > maxLines {
		truncated = true
		lines = lines[:maxLines]
		content = strings.Join(lines, "\n")
	}

	return content, truncated
}

// groupMessagesIntoBlocks converts raw messages into renderable conversation blocks.
func groupMessagesIntoBlocks(messages []llm.Message) []ConversationBlock {
	var blocks []ConversationBlock
	processed := make(map[int]bool)

	for i, msg := range messages {
		if msg.Role == "system" || processed[i] {
			continue
		}

		switch msg.Role {
		case "user":
			blocks = append(blocks, ConversationBlock{
				Type:      "user",
				Content:   msg.Content,
				IsLastMsg: i == len(messages)-1,
				MsgIndex:  i,
			})
			processed[i] = true

		case "assistant":
			// Skip empty assistant messages
			if msg.Content == "" && len(msg.ToolCalls) == 0 {
				processed[i] = true
				continue
			}

			block := ConversationBlock{
				Type:      "assistant",
				MsgIndex:  i,
				IsLastMsg: i == len(messages)-1,
			}

			// Create first segment from current message
			segment := AssistantSegment{Content: msg.Content}
			for _, tc := range msg.ToolCalls {
				tcWithResult := ToolCallWithResult{Call: tc}
				for j := i + 1; j < len(messages); j++ {
					if messages[j].Role == "tool" && messages[j].ToolCallID == tc.ID {
						tcWithResult.Result = messages[j].Content
						processed[j] = true
						break
					}
				}
				segment.ToolCalls = append(segment.ToolCalls, tcWithResult)
			}
			block.Segments = append(block.Segments, segment)
			processed[i] = true

			// Check for follow-up assistant messages in the same turn
			for j := i + 1; j < len(messages); j++ {
				if processed[j] {
					continue
				}
				if messages[j].Role == "tool" {
					continue
				}
				if messages[j].Role == "assistant" {
					// Create a new segment for this message
					seg := AssistantSegment{Content: messages[j].Content}
					for _, tc := range messages[j].ToolCalls {
						tcWithResult := ToolCallWithResult{Call: tc}
						for k := j + 1; k < len(messages); k++ {
							if messages[k].Role == "tool" && messages[k].ToolCallID == tc.ID {
								tcWithResult.Result = messages[k].Content
								processed[k] = true
								break
							}
						}
						seg.ToolCalls = append(seg.ToolCalls, tcWithResult)
					}
					block.Segments = append(block.Segments, seg)
					block.IsLastMsg = j == len(messages)-1
					processed[j] = true

					// If no tool calls, this is the final response, stop collecting
					if len(messages[j].ToolCalls) == 0 {
						break
					}
					continue
				}
				break
			}

			blocks = append(blocks, block)
		}
	}

	return blocks
}

// renderUserBlock renders a user message block.
func renderUserBlock(block ConversationBlock) string {
	var b strings.Builder
	b.WriteString(userRoleStyle.Render("You") + ":\n")
	b.WriteString(block.Content + "\n\n")
	return b.String()
}

// renderToolCallBlock renders the tool calls section with a border.
func renderToolCallBlock(toolCalls []ToolCallWithResult) string {
	var b strings.Builder

	for _, tc := range toolCalls {
		b.WriteString(toolCallStyle.Render(fmt.Sprintf("▶ 调用工具: %s", tc.Call.Function.Name)) + "\n")
		if tc.Call.Function.Arguments != "" && tc.Call.Function.Arguments != "{}" {
			b.WriteString(toolArgStyle.Render(fmt.Sprintf("  参数: %s", tc.Call.Function.Arguments)) + "\n")
		}

		if tc.Result != "" {
			b.WriteString(resultLabel.Render("◀ 结果:") + "\n")
			content, truncated := truncateContent(tc.Result, 500, 10)
			indented := strings.ReplaceAll(content, "\n", "\n   ")
			b.WriteString(resultContent.Render("   " + indented))
			if truncated {
				b.WriteString(truncatedStyle.Render("\n   ... (输出已截断)"))
			}
			b.WriteString("\n")
		}
	}

	return toolBoxStyle.Render(strings.TrimRight(b.String(), "\n"))
}

// renderAssistantBlock renders an assistant message block with ordered segments.
func renderAssistantBlock(block ConversationBlock, renderer *glamour.TermRenderer, lastContent string, isStreaming bool) string {
	var b strings.Builder
	b.WriteString(assistantRoleStyle.Render("Tachigoma") + ":\n")

	// Render each segment in order (content first, then tool calls)
	for i, seg := range block.Segments {
		isLastSegment := i == len(block.Segments)-1

		// Render text content
		if seg.Content != "" {
			if block.IsLastMsg && isLastSegment && isStreaming {
				// During streaming, show raw content
				b.WriteString(lastContent)
			} else {
				// Full render with markdown
				rendered, err := renderer.Render(seg.Content)
				if err != nil {
					rendered = seg.Content
				}
				b.WriteString(rendered)
			}
		}

		// Render tool calls if any
		if len(seg.ToolCalls) > 0 {
			if seg.Content != "" {
				b.WriteString("\n")
			}
			b.WriteString(renderToolCallBlock(seg.ToolCalls))
			b.WriteString("\n")
		}
	}

	b.WriteString("\n")
	return b.String()
}

// renderConversation renders the message history.
func (m model) renderConversation(fullRender bool) string {
	var b strings.Builder
	viewState := m.agent.GetViewState()

	// Use cached renderer for better performance (especially on SSH sessions)
	blocks := groupMessagesIntoBlocks(viewState.Messages)

	for _, block := range blocks {
		switch block.Type {
		case "user":
			b.WriteString(renderUserBlock(block))
		case "assistant":
			isStreaming := !fullRender && block.IsLastMsg
			b.WriteString(renderAssistantBlock(block, m.renderer, m.lastContent, isStreaming))
		}
	}

	// Render status line (loading indicator or error)
	if m.loading && len(m.lastContent) == 0 {
		b.WriteString(assistantRoleStyle.Render("Tachigoma") + ": ...\n")
	} else if m.err != nil {
		b.WriteString(errorStyle.Render(fmt.Sprintf("Error: %v\n", m.err)))
	}

	return b.String()
}
