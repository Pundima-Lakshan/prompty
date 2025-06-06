package models

import (
	"fmt"
	"prompty/internal/ui/styles"
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ComposeModel handles prompt composition
type ComposeModel struct {
	textarea      textarea.Model
	selectedFiles []string
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
		textarea: ta,
		selectedFiles: []string{
			"main.go",
			"internal/ui/models/app.go", // Mock selected files
		},
		finalPrompt: "",
		showOutput:  false,
	}
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
		case "ctrl+s":
			// TODO: Save prompt to file
			return m, nil
		case "ctrl+c":
			if m.showOutput {
				// TODO: Copy to clipboard
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
			builder.WriteString(fmt.Sprintf("### %s\n\n", file))
			builder.WriteString("```\n")
			builder.WriteString("// File content would be here\n")
			builder.WriteString("package main\n\n")
			builder.WriteString("func main() {\n")
			builder.WriteString("    // Sample content\n")
			builder.WriteString("}\n")
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
	for _, file := range m.selectedFiles {
		filesList = append(filesList, "  ✓ "+file)
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

	// Help section
	help := styles.HelpStyle.Render(
		"Ctrl+G: Generate • Ctrl+S: Save • Esc: Back",
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

	help := styles.HelpStyle.Render(
		"Ctrl+C: Copy • Ctrl+S: Save • Esc: Back to editing",
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
