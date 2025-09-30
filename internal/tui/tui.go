package tui

import (
	"GhOst/internal/llm"
	"fmt"
	"strings"

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
	viewport    viewport.Model
	textarea    textarea.Model
	llmClient   *llm.Client
	llmModel    string
	messages    []llm.Message
	sub         chan tea.Msg // Channel for receiving streaming messages
	loading     bool
	lastContent string // Stores the full content of the last streaming message
	err         error
}

// A command that waits for the next message from a subscription.
func waitForActivity(sub chan tea.Msg) tea.Cmd {
	return func() tea.Msg {
		return <-sub
	}
}

// NewModel creates the initial model for the TUI.
func NewModel(client *llm.Client, modelName string) tea.Model {
	ti := textarea.New()
	ti.Placeholder = ""
	ti.Focus()

	vp := viewport.New(0, 0)
	return model{
		llmClient: client,
		llmModel:  modelName,
		textarea:  ti,
		viewport:  vp,
		messages:  []llm.Message{},
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
		availableHeight := msg.Height - m.textarea.Height() - lipgloss.Height(m.helpView())
		m.viewport.Width = msg.Width
		m.viewport.Height = availableHeight
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
		m.messages = append(m.messages, llm.Message{Role: "assistant", Content: ""})
		return m, waitForActivity(m.sub) // Wait for the next message

	case llm.StreamContentMsg:
		if len(m.messages) > 0 {
			last := len(m.messages) - 1
			m.messages[last].Content += msg.Content
			m.lastContent = m.messages[last].Content
			m.viewport.SetContent(m.renderConversation(false))
			m.viewport.GotoBottom()
		}
		return m, waitForActivity(m.sub) // Wait for the next message

	case llm.StreamEndMsg:
		m.loading = false
		m.sub = nil // Stop listening
		m.viewport.SetContent(m.renderConversation(true))
		m.viewport.GotoBottom()
		return m, nil

	case llm.ErrorMsg:
		m.loading = false
		m.err = msg.Err
		m.sub = nil // Stop listening
		m.viewport.SetContent(m.renderConversation(true))
		m.viewport.GotoBottom()
		return m, nil

	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			return m, tea.Quit
		case tea.KeyEnter:
			prompt := strings.TrimSpace(m.textarea.Value())
			if prompt != "" && !m.loading {
				m.messages = append(m.messages, llm.Message{Role: "user", Content: prompt})
				m.textarea.Reset()
				m.viewport.SetContent(m.renderConversation(true))
				m.viewport.GotoBottom()
				return m, m.llmClient.CompletionStream(m.messages, m.llmModel)
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
	return lipgloss.JoinVertical(
		lipgloss.Left,
		m.viewport.View(),
		m.textarea.View(),
		m.helpView(),
	)
}

// helpView renders the help text at the bottom.
func (m model) helpView() string {
	return helpStyle.Render("enter: send | ctrl+c: quit")
}

// renderConversation renders the message history.
func (m model) renderConversation(fullRender bool) string {
	var b strings.Builder

	renderer, _ := glamour.NewTermRenderer(glamour.WithAutoStyle())

	for i, msg := range m.messages {
		var roleStyle lipgloss.Style
		var roleText string

		if msg.Role == "user" {
			roleText = "You"
			roleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("70"))
			b.WriteString(roleStyle.Render(roleText) + ":\n")
			b.WriteString(msg.Content + "\n\n")
		} else {
			roleText = "GhOst"
			roleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("66"))
			b.WriteString(roleStyle.Render(roleText) + ":\n")

			if i == len(m.messages)-1 && !fullRender {
				b.WriteString(msg.Content)
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
		b.WriteString("GhOst: ...\n")
	} else if m.err != nil {
		errorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
		b.WriteString(errorStyle.Render(fmt.Sprintf("Error: %v\n", m.err)))
	}

	return b.String()
}
