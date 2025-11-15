package tui

import (
	"fmt"
	"strings"
	"tachigoma/internal/llm"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport"
	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
)

var (
	helpStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
)

// model is the state of our TUI application.
type model struct {
	viewport        viewport.Model
	textarea        textarea.Model
	agent           *llm.Agent   // The new core logic handler
	sub             chan tea.Msg // Channel for receiving streaming messages
	loading         bool
	lastContent     string // Stores the live content of the current streaming message
	err             error
	availableHeight int  // Available height for the viewport
	ready           bool // Whether the UI has been sized and is ready for rendering
}

// --- TUI Messages ---

// A command that waits for the next message from a subscription.
func waitForActivity(sub chan tea.Msg) tea.Cmd {
	return func() tea.Msg {
		return <-sub
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

	return model{
		agent:    llm.NewAgent(client, modelName),
		textarea: ti,
		viewport: vp,
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

	// We've received the stream channel. Start listening for activity.
	case llm.Stream:
		m.sub = msg
		return m, waitForActivity(m.sub)

	case llm.StreamStartMsg:
		m.loading = true
		m.err = nil
		m.lastContent = ""
		m.agent.HandleStreamStart()
		return m, waitForActivity(m.sub)

	case llm.StreamContentMsg:
		m.agent.HandleStreamContent(msg.Content)
		m.lastContent = m.agent.GetViewState().LastStreamedContent
		m.viewport.SetContent(m.renderConversation(false))
		m.safeGotoBottom()
		return m, waitForActivity(m.sub)

	case llm.StreamEndMsg:
		m.loading = false
		m.sub = nil
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
		if m.sub != nil {
			return m, tea.Batch(cmd, waitForActivity(m.sub))
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
		if m.sub != nil {
			return m, waitForActivity(m.sub)
		}
		return m, nil

	case llm.ErrorMsg:
		m.loading = false
		m.err = msg.Err
		m.sub = nil
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
				m.loading = false
				m.sub = nil
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

// renderConversation renders the message history.
func (m model) renderConversation(fullRender bool) string {
	var b strings.Builder
	viewState := m.agent.GetViewState()

	renderer, _ := glamour.NewTermRenderer(glamour.WithAutoStyle())

	// Track which messages we've already rendered (to avoid duplicates when merging tool results)
	rendered := make(map[int]bool)

	for i, msg := range viewState.Messages {
		if msg.Role == "system" || rendered[i] {
			continue
		}

		var roleStyle lipgloss.Style
		var roleText string

		if msg.Role == "user" {
			roleText = "You"
			roleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("70"))
			b.WriteString(roleStyle.Render(roleText) + ":\n")
			b.WriteString(msg.Content + "\n\n")
			rendered[i] = true
		} else if msg.Role == "assistant" {
			// Skip empty assistant messages (e.g., during streaming setup or tool calls without content)
			if msg.Content == "" && len(msg.ToolCalls) == 0 {
				rendered[i] = true
				continue
			}

			// 收集从当前位置开始的所有连续的 assistant→tool 对，直到遇到不再有工具调用的 assistant 消息
			// 这样可以将整个工具调用链合并成一个连续的对话块
			assistantIndices := []int{i}
			j := i + 1
			for j < len(viewState.Messages) {
				// 跳过 tool 消息
				if viewState.Messages[j].Role == "tool" {
					j++
					continue
				}
				// 如果遇到下一个 assistant 消息
				if viewState.Messages[j].Role == "assistant" {
					// 如果这个 assistant 消息有工具调用，将它加入序列
					if len(viewState.Messages[j].ToolCalls) > 0 {
						assistantIndices = append(assistantIndices, j)
						j++
						continue
					} else if viewState.Messages[j].Content != "" {
						// 如果这个 assistant 消息没有工具调用但有内容，这是最终回复
						assistantIndices = append(assistantIndices, j)
						break
					}
				}
				break
			}

			// 显示 Tachigoma 标题（只显示一次）
			roleText = "Tachigoma"
			roleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("66"))
			b.WriteString(roleStyle.Render(roleText) + ":\n")

			toolCallStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("214"))      // 橙色
			toolArgStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("243"))       // 灰色
			resultLabelStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("114"))   // 浅绿色
			resultContentStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("248")) // 浅灰色

			// 遍历所有收集到的 assistant 消息
			for idx, assistantIdx := range assistantIndices {
				assistantMsg := viewState.Messages[assistantIdx]
				isLast := idx == len(assistantIndices)-1

				// 如果有文本内容，先显示文本内容
				if assistantMsg.Content != "" {
					if assistantIdx == len(viewState.Messages)-1 && !fullRender {
						b.WriteString(m.lastContent)
						if len(assistantMsg.ToolCalls) > 0 {
							b.WriteString("\n\n")
						}
					} else {
						renderedContent, err := renderer.Render(assistantMsg.Content)
						if err != nil {
							renderedContent = assistantMsg.Content
						}
						b.WriteString(renderedContent)
						if len(assistantMsg.ToolCalls) > 0 {
							b.WriteString("\n")
						}
					}
				}

				// 如果有工具调用，显示工具调用块
				if len(assistantMsg.ToolCalls) > 0 {
					var toolBlockBuilder strings.Builder

					for _, toolCall := range assistantMsg.ToolCalls {
						toolBlockBuilder.WriteString(toolCallStyle.Render(fmt.Sprintf("▶ 调用工具: %s", toolCall.Function.Name)) + "\n")
						if toolCall.Function.Arguments != "" && toolCall.Function.Arguments != "{}" {
							toolBlockBuilder.WriteString(toolArgStyle.Render(fmt.Sprintf("  参数: %s", toolCall.Function.Arguments)) + "\n")
						}

						// 查找对应的工具结果
						for k := assistantIdx + 1; k < len(viewState.Messages); k++ {
							if viewState.Messages[k].Role == "tool" && viewState.Messages[k].ToolCallID == toolCall.ID {
								toolBlockBuilder.WriteString(resultLabelStyle.Render("◀ 结果:") + "\n")
								trimmedContent := strings.TrimSpace(viewState.Messages[k].Content)

								// 截断过长的输出
								const maxLines = 10
								const maxChars = 500
								lines := strings.Split(trimmedContent, "\n")
								truncated := false

								if len(trimmedContent) > maxChars || len(lines) > maxLines {
									truncated = true
									if len(trimmedContent) > maxChars {
										trimmedContent = trimmedContent[:maxChars]
										trimmedContent = strings.TrimRight(trimmedContent, "\x80\x81\x82\x83\x84\x85\x86\x87\x88\x89\x8a\x8b\x8c\x8d\x8e\x8f")
									}
									lines = strings.Split(trimmedContent, "\n")
									if len(lines) > maxLines {
										lines = lines[:maxLines]
									}
									trimmedContent = strings.Join(lines, "\n")
								}

								indentedContent := strings.ReplaceAll(trimmedContent, "\n", "\n   ")
								toolBlockBuilder.WriteString(resultContentStyle.Render("   " + indentedContent))

								if truncated {
									truncateStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("243")).Italic(true)
									toolBlockBuilder.WriteString(truncateStyle.Render("\n   ... (输出已截断)"))
								}
								toolBlockBuilder.WriteString("\n")
								rendered[k] = true
								break
							}
						}
					}

					// 用边框包裹工具调用块
					toolBoxStyle := lipgloss.NewStyle().
						Border(lipgloss.RoundedBorder()).
						BorderForeground(lipgloss.Color("240")).
						Padding(0, 1).
						MarginLeft(2)

					toolBlock := toolBoxStyle.Render(strings.TrimRight(toolBlockBuilder.String(), "\n"))
					b.WriteString(toolBlock + "\n")
					if !isLast || assistantMsg.Content == "" {
						b.WriteString("\n")
					}
				}

				// 标记已渲染
				rendered[assistantIdx] = true
			}

			// 在对话块结尾添加空行（如果不是最后一条消息）
			if i != len(viewState.Messages)-1 {
				b.WriteString("\n")
			}
		} else if msg.Role == "tool" {
			// 如果工具消息还没被合并渲染（可能是孤立的工具结果），单独显示
			if !rendered[i] {
				resultLabelStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("114"))
				resultContentStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("248"))

				b.WriteString(resultLabelStyle.Render("  ✓ 工具结果:") + "\n")
				trimmedContent := strings.TrimSpace(msg.Content)

				// 截断过长的输出
				const maxLines = 10
				const maxChars = 500
				lines := strings.Split(trimmedContent, "\n")
				truncated := false

				if len(trimmedContent) > maxChars || len(lines) > maxLines {
					truncated = true
					if len(trimmedContent) > maxChars {
						trimmedContent = trimmedContent[:maxChars]
						trimmedContent = strings.TrimRight(trimmedContent, "\x80\x81\x82\x83\x84\x85\x86\x87\x88\x89\x8a\x8b\x8c\x8d\x8e\x8f")
					}
					lines = strings.Split(trimmedContent, "\n")
					if len(lines) > maxLines {
						lines = lines[:maxLines]
					}
					trimmedContent = strings.Join(lines, "\n")
				}

				indentedContent := strings.ReplaceAll(trimmedContent, "\n", "\n     ")
				b.WriteString(resultContentStyle.Render("     " + indentedContent))

				if truncated {
					truncateStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("243")).Italic(true)
					b.WriteString(truncateStyle.Render("\n     ... (输出已截断)"))
				}
				b.WriteString("\n\n")
				rendered[i] = true
			}
		}
	}

	if m.loading && len(m.lastContent) == 0 {
		b.WriteString("Tachigoma: ...\n")
	} else if m.err != nil {
		errorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
		b.WriteString(errorStyle.Render(fmt.Sprintf("Error: %v\n", m.err)))
	}

	return b.String()
}
