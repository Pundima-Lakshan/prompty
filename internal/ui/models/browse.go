package models

import (
	"fmt"
	"log"
	"prompty/internal/search"
	"prompty/internal/ui/styles"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// FileItem represents a file that can be browsed, tagged, and its content viewed.
// It includes Path, Content, and Tagged status.
type FileItem struct {
	Path    string // The relative path of the file
	Content string // The full content of the file (loaded lazily in SearchModel)
	Tagged  bool   // Whether the file has been tagged by the user
	// OriginalMatch is an optional field to store the RipgrepMatch that led to this file,
	// useful for context but not directly used in prompt composition.
	// We keep it here for completeness, though it's mainly populated in SearchModel.
	OriginalMatch *search.RipgrepMatch
}

// BrowseModel handles the display and management of *already tagged* files.
// It allows reviewing these files and untagging them if needed.
type BrowseModel struct {
	files       []FileItem // List of files currently displayed (these are already tagged)
	cursor      int        // Index of the currently highlighted file
	preview     string     // Content of the file currently being previewed
	showPreview bool       // Flag to indicate if the file preview is active
}

// Init initializes the browse model.
func (m *BrowseModel) Init() tea.Cmd {
	return nil
}

// NewBrowseModel creates a new browse model.
// It starts with an an empty list of files, as files are passed from the App model.
func NewBrowseModel() *BrowseModel {
	return &BrowseModel{
		files:       []FileItem{}, // Files will be set externally
		cursor:      0,
		preview:     "",
		showPreview: false,
	}
}

// SetTaggedFiles updates the BrowseModel's file list with the currently tagged files.
// This function is called by the App model when tagged files change in SearchModel.
func (m *BrowseModel) SetTaggedFiles(files []FileItem) tea.Cmd {
	log.Printf("BrowseModel: SetTaggedFiles received %d files.", len(files))
	m.files = files // Replace the current list with the new tagged files
	// Reset cursor if the list is now empty or cursor is out of bounds
	if len(m.files) == 0 {
		m.cursor = 0
	} else if m.cursor >= len(m.files) {
		m.cursor = len(m.files) - 1
	}
	// If preview was active, clear it as the file list might have changed
	if m.showPreview {
		m.showPreview = false
		m.preview = ""
	}
	return nil // No command returned
}

// Update handles messages for the BrowseModel.
// It processes keyboard input for navigation, untagging, and previewing files.
func (m *BrowseModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd                                         // To batch commands
	log.Printf("BrowseModel Update received message: %T", msg) // Log all incoming messages

	switch msg := msg.(type) {
	case tea.KeyMsg:
		log.Printf("BrowseModel: KeyMsg received: %s (Type: %d, Mod: %d)", msg.String(), msg.Type)
		switch msg.Type {
		case tea.KeyCtrlN: // Only Ctrl+N for navigating down
			log.Printf("BrowseModel: Ctrl+N pressed (down).")
			if len(m.files) > 0 {
				m.cursor = (m.cursor + 1) % len(m.files)
				log.Printf("BrowseModel: Cursor moved to %d.", m.cursor)
				// If preview is active, update it to the content of the newly selected file.
				if m.showPreview {
					m.preview = m.files[m.cursor].Content // Content should already be loaded
					log.Printf("BrowseModel: Preview updated for %s.", m.files[m.cursor].Path)
				}
			}
		case tea.KeyCtrlP: // Only Ctrl+P for navigating up
			log.Printf("BrowseModel: Ctrl+P pressed (up).")
			if len(m.files) > 0 {
				m.cursor = (m.cursor - 1 + len(m.files)) % len(m.files)
				log.Printf("BrowseModel: Cursor moved to %d.", m.cursor)
				// If preview is active, update it to the content of the newly selected file.
				if m.showPreview {
					m.preview = m.files[m.cursor].Content // Content should already be loaded
					log.Printf("BrowseModel: Preview updated for %s.", m.files[m.cursor].Path)
				}
			}
		case tea.KeyEnter:
			log.Printf("BrowseModel: Enter key pressed.")
			// Toggle preview.
			if m.cursor >= 0 && m.cursor < len(m.files) {
				if m.showPreview {
					m.showPreview = false
					m.preview = ""
					log.Printf("BrowseModel: Preview closed.")
				} else {
					m.showPreview = true
					// Display content, which should already be loaded.
					m.preview = m.files[m.cursor].Content
					log.Printf("BrowseModel: Preview opened for %s.", m.files[m.cursor].Path)
				}
			}
		case tea.KeyEsc:
			log.Printf("BrowseModel: Esc key pressed.")
			// Close preview.
			m.showPreview = false
			m.preview = ""
			log.Printf("BrowseModel: Preview closed via Esc.")
		case tea.KeyCtrlA: // Ctrl+A for untagging
			log.Printf("BrowseModel: Ctrl+A pressed (untag).")
			if m.cursor >= 0 && m.cursor < len(m.files) {
				if m.files[m.cursor].Tagged { // Only untag if it's currently tagged
					// Send a message to the App model so it can update the source of truth (SearchModel)
					// and then update the ComposeModel and *this* BrowseModel.
					cmds = append(cmds, func() tea.Msg {
						return UntagFileMsg{Path: m.files[m.cursor].Path}
					})
					log.Printf("BrowseModel: Sent UntagFileMsg for %s. Awaiting update from AppModel.", m.files[m.cursor].Path)

					// DO NOT locally modify m.files here. The App model will re-set m.files
					// via SetTaggedFiles with the correct, updated list.
					// We can, however, clear the preview immediately for better UX.
					m.showPreview = false
					m.preview = ""
					return m, tea.Batch(cmds...)
				}
			}
		}
	}

	return m, tea.Batch(cmds...) // Return batched commands if any
}

// View renders the browse interface.
func (m *BrowseModel) View() string {
	// File list
	var fileList []string
	taggedCount := len(m.files) // In this model, all files are by definition "selected/tagged"

	if taggedCount == 0 {
		fileList = append(fileList, lipgloss.NewStyle().Foreground(styles.MutedColor).Render("No files have been tagged yet. Go to 'Search' tab to find and tag files."))
	} else {
		for i, file := range m.files {
			var style lipgloss.Style
			cursor := "  "

			if i == m.cursor {
				cursor = "▶ "
				style = styles.SelectedStyle // Highlight selected item
			} else {
				style = styles.NormalStyle
			}

			// All files here are conceptually tagged, so always show a checkmark
			line := cursor + "✓ " + file.Path
			fileList = append(fileList, style.Render(line))
		}
	}

	// Main content
	title := lipgloss.NewStyle().Bold(true).Render(
		fmt.Sprintf("📋 Tagged Files (%d)", taggedCount),
	)

	files := lipgloss.JoinVertical(lipgloss.Left, fileList...)

	// Updated help text for new keybindings
	help := styles.HelpStyle.Render(
		"Ctrl+N/Ctrl+P: Navigate • Ctrl+A: Untag • Enter: Preview • Esc: Close preview",
	)

	leftPanel := lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		"",
		files,
		"",
		help,
	)

	// If preview is shown, create two-column layout
	if m.showPreview && m.preview != "" {
		previewTitle := lipgloss.NewStyle().Bold(true).Render(
			fmt.Sprintf("👁 Preview: %s", m.files[m.cursor].Path),
		)

		previewContent := lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(styles.MutedColor).
			Padding(1).
			Width(60).
			Height(15).
			Render(m.preview)

		rightPanel := lipgloss.JoinVertical(
			lipgloss.Left,
			previewTitle,
			"",
			previewContent,
		)

		return lipgloss.JoinHorizontal(
			lipgloss.Top,
			leftPanel,
			"  ",
			rightPanel,
		)
	}

	return leftPanel
}
