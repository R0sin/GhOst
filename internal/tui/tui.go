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

// Custom messages for Bubble Tea
type llmResponseMsg string
type errorMsg struct{ err error }

var (
	helpStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
)

// model is the state of our TUI application.
type model struct {
	viewport  viewport.Model
	textarea  textarea.Model
	llmClient *llm.Client
	llmModel  string
	messages  []llm.Message
	loading   bool
	err       error
}

// NewModel creates the initial model for the TUI.
func NewModel(client *llm.Client, modelName string) tea.Model {
	ti := textarea.New()
	ti.Placeholder = ""
	ti.Focus()

	// The viewport will be initialized with the correct size via a WindowSizeMsg.
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

// waitForLLMResponse is a command that calls the LLM API and returns the response.
func (m model) waitForLLMResponse() tea.Msg {
	resp, err := m.llmClient.Completion(m.messages, m.llmModel)
	if err != nil {
		return errorMsg{err}
	}
	return llmResponseMsg(resp)
}

// Update handles incoming messages and updates the model accordingly.
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		availableHeight := msg.Height - m.textarea.Height() - lipgloss.Height(m.helpView())
		viewportHeight := availableHeight
		m.viewport.Width = msg.Width
		m.viewport.Height = viewportHeight
		m.textarea.SetWidth(msg.Width)
		return m, nil // Return early

	case llmResponseMsg:
		m.loading = false
		m.messages = append(m.messages, llm.Message{Role: "assistant", Content: string(msg)})
		m.viewport.SetContent(m.renderConversation())
		m.viewport.GotoBottom()

	case errorMsg:
		m.loading = false
		m.err = msg.err
		m.viewport.SetContent(m.renderConversation())
		m.viewport.GotoBottom()

	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			return m, tea.Quit
		case tea.KeyEnter:
			// Get the value, but trim the newline that the component adds by default.
			prompt := strings.TrimSpace(m.textarea.Value())
			if prompt != "" && !m.loading {
				m.loading = true
				m.messages = append(m.messages, llm.Message{Role: "user", Content: prompt})
				m.textarea.Reset()
				m.viewport.SetContent(m.renderConversation())
				m.viewport.GotoBottom()
				// We need to manually re-add the llm response handling
				return m, m.waitForLLMResponse
			}
		}
	}

	// Pass all messages to child components.
	m.textarea, cmd = m.textarea.Update(msg)
	cmds = append(cmds, cmd)

	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

// View renders the UI based on the model's state.
func (m model) View() string {
	// lipgloss.JoinVertical arranges strings vertically.
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

// renderConversation renders the entire message history into a single string.
func (m model) renderConversation() string {
	var b strings.Builder

	renderer, _ := glamour.NewTermRenderer(glamour.WithAutoStyle())

	for _, msg := range m.messages {
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
			renderedContent, err := renderer.Render(msg.Content)
			if err != nil {
				renderedContent = msg.Content
			}
			b.WriteString(renderedContent + "\n")
		}
	}

	if m.loading {
		b.WriteString("GhOst: ...\n")
	} else if m.err != nil {
		errorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
		b.WriteString(errorStyle.Render(fmt.Sprintf("Error: %v\n", m.err)))
	}

	return b.String()
}
