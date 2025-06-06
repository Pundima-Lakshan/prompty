package models

import (
	"fmt"
	"prompty/internal/ui/styles"
	"strings"

	"github.com/atotto/clipboard" // Added: Import the clipboard library
	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ComposeModel handles prompt composition
type ComposeModel struct {
	textarea      textarea.Model
	selectedFiles []FileItem
	finalPrompt   string
	showOutput    bool
}

// Init initializes the compose model
func (m *ComposeModel) Init() tea.Cmd {
	return textarea.Blink
}

// NewComposeModel creates a new compose model
func NewComposeModel() *ComposeModel {
	ta := textarea.New()
	ta.Placeholder = "Enter your prompt here...\n\nExample: 'Please review this code and suggest improvements'"
	ta.Focus()
	ta.SetWidth(80)
	ta.SetHeight(8)

	return &ComposeModel{
		textarea:      ta,
		selectedFiles: []FileItem{}, // Populated by App model
		finalPrompt:   "",
		showOutput:    false,
	}
}

// SetSelectedFiles updates the ComposeModel's list of files that will be included
// in the generated prompt. This method is called by the App model.
func (m *ComposeModel) SetSelectedFiles(files []FileItem) tea.Cmd {
	m.selectedFiles = files // Update the list of selected files
	// If the output screen is currently visible, regenerate the prompt to reflect
	// any changes in the selected files' content or list.
	if m.showOutput {
		m.generatePrompt()
	}
	return nil // No command returned
}

// Update handles compose model updates
func (m *ComposeModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+g":
			// Generate final prompt
			m.generatePrompt()
			m.showOutput = true
			return m, nil
		case "esc":
			if m.showOutput {
				m.showOutput = false
				return m, nil
			}
		case "y": // Copy to clipboard
			if m.showOutput {
				// Implement copy to clipboard functionality using github.com/atotto/clipboard
				err := clipboard.WriteAll(m.finalPrompt)
				if err != nil {
					// In a real application, you might want to show an error message to the user
					// For now, we'll just acknowledge the attempt.
					// You could add a temporary status message to the model for this.
					_ = err // Suppress "err declared and not used" warning if not handled visibly
				}
				return m, nil
			}
		}
	}

	m.textarea, cmd = m.textarea.Update(msg)
	return m, cmd
}

// generatePrompt creates the final prompt with selected files
func (m *ComposeModel) generatePrompt() {
	userPrompt := strings.TrimSpace(m.textarea.Value())

	var builder strings.Builder

	// Add user prompt
	if userPrompt != "" {
		builder.WriteString("## User Request\n\n")
		builder.WriteString(userPrompt)
		builder.WriteString("\n\n")
	}

	// Add selected files
	if len(m.selectedFiles) > 0 {
		builder.WriteString("## Relevant Files\n\n")

		for _, file := range m.selectedFiles {
			builder.WriteString(fmt.Sprintf("### %s\n\n", file.Path))
			builder.WriteString("```\n")
			builder.WriteString(file.Content) // Use actual file content
			builder.WriteString("```\n\n")
		}
	}

	m.finalPrompt = builder.String()
}

// View renders the compose interface
func (m *ComposeModel) View() string {
	if m.showOutput {
		return m.renderOutput()
	}

	// Selected files section
	filesTitle := lipgloss.NewStyle().Bold(true).Render(
		fmt.Sprintf("📋 Selected Files (%d)", len(m.selectedFiles)),
	)

	var filesList []string
	if len(m.selectedFiles) == 0 {
		filesList = append(filesList, styles.HelpStyle.Render("No files selected yet. Go to 'Search' tab to find and tag files."))
	} else {
		for _, file := range m.selectedFiles {
			filesList = append(filesList, "  ✓ "+file.Path)
		}
	}

	filesSection := lipgloss.JoinVertical(
		lipgloss.Left,
		filesTitle,
		"",
		lipgloss.JoinVertical(lipgloss.Left, filesList...),
	)

	// Prompt input section
	promptTitle := lipgloss.NewStyle().Bold(true).Render("✍️  Your Prompt")
	promptSection := lipgloss.JoinVertical(
		lipgloss.Left,
		promptTitle,
		"",
		m.textarea.View(),
	)

	// Help section - Removed "Ctrl+S: Save"
	help := styles.HelpStyle.Render(
		"Ctrl+G: Generate • Esc: Back",
	)

	return lipgloss.JoinVertical(
		lipgloss.Left,
		filesSection,
		"",
		"",
		promptSection,
		"",
		help,
	)
}

// renderOutput shows the final generated prompt
func (m *ComposeModel) renderOutput() string {
	title := lipgloss.NewStyle().Bold(true).Render("🎯 Generated Prompt")

	content := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(styles.SecondaryColor).
		Padding(1).
		Width(100).
		Height(20).
		Render(m.finalPrompt)

	// Updated help text to include 'Y' for copy
	help := styles.HelpStyle.Render(
		"Y: Copy • Esc: Back to editing",
	)

	return lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		"",
		content,
		"",
		help,
	)
}
