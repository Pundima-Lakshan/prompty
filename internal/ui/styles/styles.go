package styles

import (
	"github.com/charmbracelet/lipgloss"
)

var (
	// Colors used throughout the application for a consistent theme.
	PrimaryColor    = lipgloss.Color("#7C3AED") // Purple
	SecondaryColor  = lipgloss.Color("#10B981") // Green
	AccentColor     = lipgloss.Color("#F59E0B") // Amber
	ErrorColor      = lipgloss.Color("#EF4444") // Red
	MutedColor      = lipgloss.Color("#6B7280") // Gray for less prominent text
	BackgroundColor = lipgloss.Color("#1F2937") // Dark blue-gray for backgrounds

	// BaseStyle for general content containers.
	BaseStyle = lipgloss.NewStyle().
			Padding(1, 2).                    // Padding around content
			Border(lipgloss.RoundedBorder()). // Rounded border for a modern look
			BorderForeground(MutedColor)      // Muted border color

	// HeaderStyle for the main application title.
	HeaderStyle = lipgloss.NewStyle().
			Bold(true).                           // Bold text
			Foreground(PrimaryColor).             // Primary color for text
			BorderStyle(lipgloss.NormalBorder()). // Normal border style
			BorderBottom(true).                   // Only bottom border
			BorderForeground(PrimaryColor).       // Primary color for border
			MarginBottom(1).                      // Margin below the header
			Padding(0, 1)                         // Padding within the header

	// StatusStyle for status bar elements (currently not explicitly used as a bar but for concepts).
	StatusStyle = lipgloss.NewStyle().
			Background(PrimaryColor).              // Primary color background
			Foreground(lipgloss.Color("#FFFFFF")). // White text
			Padding(0, 1)                          // Padding

	// HelpStyle for hints and keyboard shortcuts.
	HelpStyle = lipgloss.NewStyle().
			Foreground(MutedColor). // Muted text color
			Italic(true)            // Italic font

	// SelectedStyle for the currently highlighted item in lists.
	SelectedStyle = lipgloss.NewStyle().
			Background(PrimaryColor).              // Primary color background
			Foreground(lipgloss.Color("#FFFFFF")). // White text
			Bold(true)                             // Bold text

	// NormalStyle for unselected items in lists.
	NormalStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFFFFF")) // White text

	// TaggedStyle for items that have been "tagged" or selected for inclusion.
	TaggedStyle = lipgloss.NewStyle().
			Background(SecondaryColor).            // Secondary color background
			Foreground(lipgloss.Color("#FFFFFF")). // White text
			Bold(true)                             // Bold text

	// Tab styles for navigation.
	ActiveTabStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(PrimaryColor).
			Padding(0, 2).
			MarginRight(1)

	InactiveTabStyle = lipgloss.NewStyle().
				Foreground(MutedColor).
				Background(lipgloss.Color("#374151")). // Slightly darker gray for inactive tabs
				Padding(0, 2).
				MarginRight(1)

	// TabBarStyle for the container holding all tabs.
	TabBarStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.NormalBorder()). // Normal border
			BorderBottom(true).                   // Only bottom border
			BorderForeground(MutedColor).         // Muted border color
			MarginBottom(1)                       // Margin below the tab bar

	// Individual tab colors for visual distinction.
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
