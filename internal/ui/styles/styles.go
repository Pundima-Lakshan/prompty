package styles

import (
	"github.com/charmbracelet/lipgloss"
)

var (
	// Colors
	PrimaryColor    = lipgloss.Color("#7C3AED")
	SecondaryColor  = lipgloss.Color("#10B981")
	AccentColor     = lipgloss.Color("#F59E0B")
	ErrorColor      = lipgloss.Color("#EF4444")
	MutedColor      = lipgloss.Color("#6B7280")
	BackgroundColor = lipgloss.Color("#1F2937")

	// Base styles
	BaseStyle = lipgloss.NewStyle().
			Padding(1, 2).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(MutedColor)

	// Header style
	HeaderStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(PrimaryColor).
			BorderStyle(lipgloss.NormalBorder()).
			BorderBottom(true).
			BorderForeground(PrimaryColor).
			MarginBottom(1).
			Padding(0, 1)

	// Status bar
	StatusStyle = lipgloss.NewStyle().
			Background(PrimaryColor).
			Foreground(lipgloss.Color("#FFFFFF")).
			Padding(0, 1)

	// Help text
	HelpStyle = lipgloss.NewStyle().
			Foreground(MutedColor).
			Italic(true)

	// Selected item
	SelectedStyle = lipgloss.NewStyle().
			Background(PrimaryColor).
			Foreground(lipgloss.Color("#FFFFFF")).
			Bold(true)

	// Normal item
	NormalStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFFFF"))

	// Tagged item
	TaggedStyle = lipgloss.NewStyle().
			Background(SecondaryColor).
			Foreground(lipgloss.Color("#FFFFFF")).
			Bold(true)

	// Tab styles
	ActiveTabStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(PrimaryColor).
			Padding(0, 2).
			MarginRight(1)

	InactiveTabStyle = lipgloss.NewStyle().
				Foreground(MutedColor).
				Background(lipgloss.Color("#374151")).
				Padding(0, 2).
				MarginRight(1)

	TabBarStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.NormalBorder()).
			BorderBottom(true).
			BorderForeground(MutedColor).
			MarginBottom(1)

	// Individual tab colors
	SearchTabStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(lipgloss.Color("#3B82F6")). // Blue
			Padding(0, 2).
			MarginRight(1)

	BrowseTabStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(lipgloss.Color("#10B981")). // Green
			Padding(0, 2).
			MarginRight(1)

	ComposeTabStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(lipgloss.Color("#F59E0B")). // Amber
			Padding(0, 2).
			MarginRight(1)
)
