package models

import (
	"fmt"
	"prompty/internal/search" // Added: Import for search.RipgrepMatch
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
	OriginalMatch *search.RipgrepMatch // Uncommented: This field is needed by SearchModel
}

// BrowseModel handles the display and management of *already tagged* files.
// It allows reviewing these files and untagging them if needed.
type BrowseModel struct {
	files       []FileItem // List of files currently displayed (these are already tagged)
	cursor      int        // Index of the currently highlighted file
	preview     string     // Content of the file currently being previewed
	showPreview bool       // Flag to indicate if the file preview is active
	// baseDir is not strictly needed here as content is expected to be loaded by SearchModel
	// baseDir     string
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
	var cmd tea.Cmd // Placeholder for commands to be returned

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type { // Changed from msg.String() to msg.Type
		case tea.KeyCtrlN: // Only Ctrl+N for navigating down
			if len(m.files) > 0 {
				m.cursor = (m.cursor + 1) % len(m.files)
				// If preview is active, update it to the content of the newly selected file.
				if m.showPreview {
					m.preview = m.files[m.cursor].Content // Content should already be loaded
				}
			}
		case tea.KeyCtrlP: // Only Ctrl+P for navigating up
			if len(m.files) > 0 {
				m.cursor = (m.cursor - 1 + len(m.files)) % len(m.files)
				// If preview is active, update it to the content of the newly selected file.
				if m.showPreview {
					m.preview = m.files[m.cursor].Content // Content should already be loaded
				}
			}
		case tea.KeyEnter:
			// Toggle preview.
			if m.cursor >= 0 && m.cursor < len(m.files) {
				if m.showPreview {
					m.showPreview = false
					m.preview = ""
				} else {
					m.showPreview = true
					// Display content, which should already be loaded.
					m.preview = m.files[m.cursor].Content
				}
			}
		case tea.KeyEsc:
			// Close preview.
			m.showPreview = false
			m.preview = ""
		case tea.KeyCtrlJ: // Changed: Now checking for tea.KeyCtrlJ
			// Toggle tag on current file (effectively untagging in this view).
			if m.cursor >= 0 && m.cursor < len(m.files) {
				if m.files[m.cursor].Tagged {
					m.files[m.cursor].Tagged = false // Mark as untagged

					// Send a message to the App model so it can update the source of truth (SearchModel)
					// and then update the ComposeModel.
					cmds := []tea.Cmd{func() tea.Msg {
						// This message needs to be handled by App to correctly update the source of truth.
						// We'll indicate which file to untag.
						return UntagFileMsg{Path: m.files[m.cursor].Path}
					}}
					// Remove the untagged file from this list immediately for visual feedback
					m.files = append(m.files[:m.cursor], m.files[m.cursor+1:]...)
					if m.cursor >= len(m.files) && len(m.files) > 0 {
						m.cursor = len(m.files) - 1
					} else if len(m.files) == 0 {
						m.cursor = 0
					}
					// If preview was active, clear it.
					m.showPreview = false
					m.preview = ""
					return m, tea.Batch(cmds...)
				}
			}
		}
	}

	return m, cmd
}

// View renders the browse interface.
func (m *BrowseModel) View() string {
	// File list
	var fileList []string
	taggedCount := len(m.files) // In this model, all files are by definition "selected/tagged"

	if taggedCount == 0 {
		// Fix for styles.MutedColor.Render undefined: Create a new style.
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
		"Ctrl+N/Ctrl+P: Navigate • Ctrl+J: Untag • Enter: Preview • Esc: Close preview",
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
