package models

import (
	"fmt"
	"prompty/internal/ui/styles"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// SearchModel handles the search functionality
type SearchModel struct {
	textInput textinput.Model
	results   []string
	focused   bool
}

// Init initializes the search model
func (m *SearchModel) Init() tea.Cmd {
	return textinput.Blink
}

// NewSearchModel creates a new search model
func NewSearchModel() *SearchModel {
	ti := textinput.New()
	ti.Placeholder = "Enter search pattern (e.g., '*.go', 'TODO', 'func main')"
	ti.Focus()
	ti.CharLimit = 156
	ti.Width = 50

	return &SearchModel{
		textInput: ti,
		results:   []string{},
		focused:   true,
	}
}

// Update handles search model updates
func (m *SearchModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			// TODO: Trigger search with ripgrep
			query := m.textInput.Value()
			if query != "" {
				// Mock results for now
				m.results = []string{
					"main.go: func main() {",
					"internal/search/ripgrep.go: // TODO: implement ripgrep",
					"README.md: # Prompt Generator",
					"go.mod: module prompt-generator",
				}
			}
			return m, nil
		}
	}

	m.textInput, cmd = m.textInput.Update(msg)
	return m, cmd
}

// View renders the search interface
func (m *SearchModel) View() string {
	// Search input section
	searchSection := lipgloss.JoinVertical(
		lipgloss.Left,
		lipgloss.NewStyle().Bold(true).Render("🔍 Search Files"),
		"",
		m.textInput.View(),
		"",
		styles.HelpStyle.Render("Enter: Search • Use ripgrep patterns like '*.go', 'TODO', etc."),
	)

	// Results section
	var resultsSection string
	if len(m.results) > 0 {
		resultsTitle := lipgloss.NewStyle().Bold(true).Render("📄 Search Results")

		var resultsList []string
		for i, result := range m.results {
			prefix := "  "
			if i < 9 {
				prefix = lipgloss.NewStyle().
					Foreground(styles.MutedColor).
					Render(fmt.Sprintf("%d. ", i+1))
			}
			resultsList = append(resultsList, prefix+result)
		}

		resultsContent := lipgloss.JoinVertical(lipgloss.Left, resultsList...)

		resultsSection = lipgloss.JoinVertical(
			lipgloss.Left,
			"",
			"",
			resultsTitle,
			"",
			resultsContent,
			"",
			styles.HelpStyle.Render("Navigate to BROWSE tab to select files"),
		)
	}

	return lipgloss.JoinVertical(
		lipgloss.Left,
		searchSection,
		resultsSection,
	)
}
