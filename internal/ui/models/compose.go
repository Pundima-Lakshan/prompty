package models

import (
	"fmt"
	"log"
	"prompty/internal/ui/styles"
	"strings"

	"github.com/atotto/clipboard"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/viewport" // Added: Import the viewport library
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ComposeModel handles prompt composition
type ComposeModel struct {
	textarea      textarea.Model
	selectedFiles []FileItem
	finalPrompt   string
	showOutput    bool
	viewport      viewport.Model // Added: Viewport for scrollable output
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
	// Initial arbitrary dimensions for textarea, will be updated by WindowSizeMsg
	ta.SetWidth(80)
	ta.SetHeight(8)

	// Initialize viewport with arbitrary dimensions, will be updated by WindowSizeMsg
	vp := viewport.New(80, 20)
	vp.HighPerformanceRendering = false // Can set to true for performance, but might redraw more often

	return &ComposeModel{
		textarea:      ta,
		selectedFiles: []FileItem{}, // Populated by App model
		finalPrompt:   "",
		showOutput:    false,
		viewport:      vp, // Initialize the viewport
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
	var cmds []tea.Cmd // To batch commands
	log.Printf("ComposeModel Update received message: %T", msg)

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		log.Printf("ComposeModel: WindowSizeMsg received. Width: %d, Height: %d", msg.Width, msg.Height)
		// Calculate available dimensions for content area (adjust for borders/padding of BaseStyle and internal UI)
		// Assuming BaseStyle takes up 2 units on each side (border + padding) and other UI elements
		contentWidth := msg.Width - 4 // For overall BaseStyle padding/borders

		// Estimate height used by fixed UI elements in the compose tab (titles, help, spacing)
		// Selected files section: depends on number of files, but has a title and spacer
		// Prompt input section: title and spacer
		// Bottom help: one line
		// Let's reserve 10 lines for these fixed elements as a rough estimate
		minFixedUiHeight := 10 // Approximate fixed height for titles, help, spacers

		availableContentHeight := msg.Height - minFixedUiHeight
		if availableContentHeight < 5 { // Ensure minimum height
			availableContentHeight = 5
		}

		if !m.showOutput {
			// When in input mode, adjust textarea size
			m.textarea.SetWidth(contentWidth)
			// Textarea height is a fixed proportion or minimum
			m.textarea.SetHeight(availableContentHeight / 2) // Example: half of available content height
			log.Printf("ComposeModel: Resized textarea to W:%d H:%d", m.textarea.Width(), m.textarea.Height())
		} else {
			// When in output mode, adjust viewport size
			m.viewport.Width = contentWidth
			m.viewport.Height = availableContentHeight
			log.Printf("ComposeModel: Resized viewport to W:%d H:%d", m.viewport.Width, m.viewport.Height)
		}
		// Also update textarea and viewport with the WindowSizeMsg so they can re-render internally
		m.textarea, cmd = m.textarea.Update(msg)
		cmds = append(cmds, cmd)
		m.viewport, cmd = m.viewport.Update(msg)
		cmds = append(cmds, cmd)
		return m, tea.Batch(cmds...)

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
				} else {
					log.Printf("ComposeModel: Prompt copied to clipboard successfully.")
				}
				return m, nil
			}
		}
	}

	// Delegate messages to active sub-components
	if m.showOutput {
		m.viewport, cmd = m.viewport.Update(msg)
		cmds = append(cmds, cmd)
	} else {
		m.textarea, cmd = m.textarea.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
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
	// Set the generated prompt content to the viewport
	m.viewport.SetContent(m.finalPrompt)
	log.Printf("ComposeModel: Final prompt generated. Total length: %d. Viewport content set.", len(m.finalPrompt))
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

// renderOutput shows the final generated prompt with scrollable viewport
func (m *ComposeModel) renderOutput() string {
	title := lipgloss.NewStyle().Bold(true).Render("🎯 Generated Prompt")

	// Render the viewport instead of direct string content
	contentView := m.viewport.View()

	// Updated help text to include 'Y' for copy and scrolling
	help := styles.HelpStyle.Render(
		"Y: Copy • Esc: Back to editing • Scroll with Up/Down Arrows, PageUp/PageDown",
	)

	return lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		"",
		contentView, // Use the viewport's rendered content
		"",
		help,
	)
}
