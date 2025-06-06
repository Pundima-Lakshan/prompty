package models

import (
	"fmt"
	"prompty/internal/ui/styles"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// FileItem represents a file in the browser
type FileItem struct {
	Path    string
	Content string
	Tagged  bool
}

// BrowseModel handles file browsing and selection
type BrowseModel struct {
	files       []FileItem
	cursor      int
	tagged      map[int]bool
	preview     string
	showPreview bool
}

// Init initializes the browse model
func (m *BrowseModel) Init() tea.Cmd {
	return nil
}

// NewBrowseModel creates a new browse model
func NewBrowseModel() *BrowseModel {
	// Mock file data for demonstration
	files := []FileItem{
		{Path: "main.go", Content: "package main\n\nimport (\n\t\"fmt\"\n\t\"log\"\n\t\"os\"\n...)"},
		{Path: "internal/search/ripgrep.go", Content: "package search\n\n// TODO: implement ripgrep integration"},
		{Path: "README.md", Content: "# Prompt Generator\n\nA CLI tool for generating prompts..."},
		{Path: "go.mod", Content: "module prompt-generator\n\ngo 1.21\n\nrequire (...)"},
		{Path: "internal/ui/models/app.go", Content: "package models\n\nimport (\n\t\"fmt\"\n...)"},
	}

	return &BrowseModel{
		files:       files,
		cursor:      0,
		tagged:      make(map[int]bool),
		preview:     "",
		showPreview: false,
	}
}

// Update handles browse model updates
func (m *BrowseModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "j", "down":
			if m.cursor < len(m.files)-1 {
				m.cursor++
			}
		case "k", "up":
			if m.cursor > 0 {
				m.cursor--
			}
		case " ":
			// Toggle tag on current file
			m.tagged[m.cursor] = !m.tagged[m.cursor]
		case "enter":
			// Toggle preview
			if m.cursor < len(m.files) {
				if m.showPreview {
					m.showPreview = false
					m.preview = ""
				} else {
					m.showPreview = true
					m.preview = m.files[m.cursor].Content
				}
			}
		case "esc":
			// Close preview
			m.showPreview = false
			m.preview = ""
		}
	}

	return m, nil
}

// View renders the browse interface
func (m *BrowseModel) View() string {
	// File list
	var fileList []string
	taggedCount := 0

	for i, file := range m.files {
		var style lipgloss.Style
		cursor := "  "

		if i == m.cursor {
			cursor = "▶ "
			if m.tagged[i] {
				style = styles.TaggedStyle
				taggedCount++
			} else {
				style = styles.SelectedStyle
			}
		} else if m.tagged[i] {
			style = styles.TaggedStyle
			taggedCount++
		} else {
			style = styles.NormalStyle
		}

		// Add tag indicator
		tag := "  "
		if m.tagged[i] {
			tag = "✓ "
		}

		line := cursor + tag + file.Path
		fileList = append(fileList, style.Render(line))
	}

	// Main content
	title := lipgloss.NewStyle().Bold(true).Render(
		fmt.Sprintf("📁 File Browser (%d selected)", taggedCount),
	)

	files := lipgloss.JoinVertical(lipgloss.Left, fileList...)

	help := styles.HelpStyle.Render(
		"j/k: Navigate • Space: Tag/Untag • Enter: Preview • Esc: Close preview",
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
