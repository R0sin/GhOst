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
	availableHeight int // Available height for the viewport
}

// --- TUI Messages ---

// A command that waits for the next message from a subscription.
func waitForActivity(sub chan tea.Msg) tea.Cmd {
	return func() tea.Msg {
		return <-sub
	}
}

// toolResultMsg is sent when a tool has finished executing.
// It is defined in the llm package but handled here.

// --- TUI Commands ---

// NewModel creates the initial model for the TUI.
func NewModel(client *llm.Client, modelName string) tea.Model {
	ti := textarea.New()
	ti.Placeholder = ""
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
		m.viewport.GotoBottom()
		return m, waitForActivity(m.sub)

	case llm.StreamEndMsg:
		m.loading = false
		m.sub = nil
		m.lastContent = ""
		m.viewport.SetContent(m.renderConversation(true))
		m.viewport.GotoBottom()
		return m, nil

	case llm.AssistantToolCallMsg:
		cmd = m.agent.HandleToolCallRequest(msg)
		m.viewport.SetContent(m.renderConversation(true))
		m.viewport.GotoBottom()
		return m, cmd

	case llm.ToolResultMsg:
		cmd = m.agent.HandleToolResult(msg.ToolCallID, msg.Result)
		m.viewport.SetContent(m.renderConversation(true))
		m.viewport.GotoBottom()
		return m, cmd

	case llm.ErrorMsg:
		m.loading = false
		m.err = msg.Err
		m.sub = nil
		m.viewport.SetContent(m.renderConversation(true))
		m.viewport.GotoBottom()
		return m, nil

	case tea.KeyMsg:
		viewState := m.agent.GetViewState()
		if viewState.IsConfirming {
			switch msg.String() {
			case "y", "Y":
				cmd = m.agent.HandleConfirmation(true)
				return m, cmd
			case "n", "N":
				cmd = m.agent.HandleConfirmation(false)
				return m, cmd
			}
		}

		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			return m, tea.Quit
		case tea.KeyEnter:
			prompt := strings.TrimSpace(m.textarea.Value())
			if prompt != "" && !m.loading && !viewState.IsConfirming {
				cmd = m.agent.HandleUserInput(prompt)
				m.textarea.Reset()
				m.viewport.SetContent(m.renderConversation(true))
				m.viewport.GotoBottom()
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

	// Dynamically set the viewport height based on whether the confirmation box is visible.
	confirmationBoxHeight := lipgloss.Height(confirmationBox)
	m.viewport.Height = m.availableHeight - confirmationBoxHeight

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
		return helpStyle.Render("y: confirm | n: deny | ctrl+c: quit")
	}
	return helpStyle.Render("enter: send | ctrl+c: quit")
}

// renderConversation renders the message history.
func (m model) renderConversation(fullRender bool) string {
	var b strings.Builder
	viewState := m.agent.GetViewState()

	renderer, _ := glamour.NewTermRenderer(glamour.WithAutoStyle())

	for i, msg := range viewState.Messages {
		var roleStyle lipgloss.Style
		var roleText string

		if msg.Role == "user" {
			roleText = "You"
			roleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("70"))
			b.WriteString(roleStyle.Render(roleText) + ":\n")
			b.WriteString(msg.Content + "\n\n")
		} else {
			roleText = "Tachigoma"
			roleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("66"))
			b.WriteString(roleStyle.Render(roleText) + ":\n")

			if i == len(viewState.Messages)-1 && !fullRender {
				b.WriteString(m.lastContent) // Use the live content for the last message
			} else {
				renderedContent, err := renderer.Render(msg.Content)
				if err != nil {
					renderedContent = msg.Content
				}
				b.WriteString(renderedContent + "\n")
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
