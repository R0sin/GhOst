package tui

import (
	"GhOst/internal/llm"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Custom messages for Bubble Tea
type llmResponseMsg string
type errorMsg struct{ err error }

// model is the state of our TUI application.
type model struct {
	llmClient *llm.Client
	llmModel  string
	messages  []llm.Message
	textarea  textarea.Model
	loading   bool
	err       error
}

// NewModel creates the initial model for the TUI.
func NewModel(client *llm.Client, modelName string) tea.Model {
	ti := textarea.New()
	ti.Placeholder = "Ask GhOst something... (Press Enter to send)"
	ti.Focus()

	return model{
		llmClient: client,
		llmModel:  modelName,
		textarea:  ti,
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
	// Handle key presses
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			return m, tea.Quit

		case tea.KeyEnter:
			if m.textarea.Value() != "" && !m.loading {
				m.loading = true
				m.messages = append(m.messages, llm.Message{Role: "user", Content: m.textarea.Value()})
				m.textarea.Reset()
				return m, m.waitForLLMResponse
			}
		}

	// Handle API response
	case llmResponseMsg:
		m.loading = false
		m.messages = append(m.messages, llm.Message{Role: "assistant", Content: string(msg)})

	// Handle API error
	case errorMsg:
		m.loading = false
		m.err = msg.err
	}

	// Pass all other messages (including key presses other than Enter/Ctrl+C) to the textarea.
	m.textarea, cmd = m.textarea.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

// View renders the UI based on the model's state.
func (m model) View() string {
	var b strings.Builder

	// Render conversation history
	for _, msg := range m.messages {
		var roleStyle lipgloss.Style
		var roleText string

		if msg.Role == "user" {
			roleText = "You"
			roleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("70")) // Purple
		} else {
			roleText = "GhOst"
			roleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("66")) // Blue
		}

		b.WriteString(roleStyle.Render(roleText) + ":\n")
		b.WriteString(msg.Content + "\n\n")
	}

	// Render loading state or error
	if m.loading {
		b.WriteString("GhOst: ...\n")
	} else if m.err != nil {
		errorStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("9")) // Red
		b.WriteString(errorStyle.Render(fmt.Sprintf("Error: %v", m.err)))
	}

	// Render textarea and help text
	b.WriteString("\n" + m.textarea.View() + "\n\n")
	b.WriteString("(Ctrl+C to quit)")

	return b.String()
}
