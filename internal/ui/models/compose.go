package models

import (
	"fmt"
	"log" // Re-enabled logging
	"prompty/internal/ui/styles"
	"strings"

	"github.com/atotto/clipboard"
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
	log.Printf("ComposeModel: SetSelectedFiles received %d files.", len(files))
	m.selectedFiles = files // Update the list of selected files
	// If the output screen is currently visible, regenerate the prompt to reflect
	// any changes in the selected files' content or list.
	if m.showOutput {
		m.generatePrompt()
		log.Printf("ComposeModel: Regenerating prompt because output was shown.")
	}
	return nil // No command returned
}

// Update handles compose model updates
func (m *ComposeModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	log.Printf("ComposeModel Update received message: %T", msg) // Log all incoming messages

	switch msg := msg.(type) {
	case tea.KeyMsg:
		log.Printf("ComposeModel: KeyMsg received: %s (Type: %d)", msg.String(), msg.Type)
		switch msg.String() {
		case "ctrl+g":
			log.Printf("ComposeModel: Ctrl+G pressed (generate).")
			// Generate final prompt
			m.generatePrompt()
			m.showOutput = true
			log.Printf("ComposeModel: Prompt generated, showing output.")
			return m, nil
		case "esc":
			log.Printf("ComposeModel: Esc key pressed.")
			if m.showOutput {
				m.showOutput = false
				log.Printf("ComposeModel: Hiding output, returning to editing.")
				return m, nil
			}
		case "y": // Copy to clipboard
			log.Printf("ComposeModel: Y key pressed (copy).")
			if m.showOutput {
				err := clipboard.WriteAll(m.finalPrompt)
				if err != nil {
					log.Printf("ComposeModel: Error copying to clipboard: %v", err)
					// In a real application, you might want to show an error message to the user
					// For now, we'll just acknowledge the attempt.
					// You could add a temporary status message to the model for this.
				} else {
					log.Printf("ComposeModel: Prompt copied to clipboard successfully.")
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
	log.Printf("ComposeModel: generatePrompt called. User prompt length: %d", len(userPrompt))

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
		log.Printf("ComposeModel: Adding %d selected files to prompt.", len(m.selectedFiles))

		for _, file := range m.selectedFiles {
			builder.WriteString(fmt.Sprintf("### %s\n\n", file.Path))
			builder.WriteString("```\n")
			builder.WriteString(file.Content) // Use actual file content
			builder.WriteString("```\n\n")
			log.Printf("ComposeModel: Added file '%s' content (length: %d) to prompt.", file.Path, len(file.Content))
		}
	} else {
		log.Printf("ComposeModel: No selected files to add.")
	}

	m.finalPrompt = builder.String()
	log.Printf("ComposeModel: Final prompt generated. Total length: %d", len(m.finalPrompt))
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

	// Help section
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
