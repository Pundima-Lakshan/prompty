package models

import (
	"fmt"
	"prompty/internal/ui/styles"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// AppState represents the current state of the application
type AppState int

const (
	SearchState AppState = iota
	BrowseState
	ComposeState
)

// App is the main application model
type App struct {
	state        AppState
	width        int
	height       int
	searchModel  *SearchModel
	browseModel  *BrowseModel
	composeModel *ComposeModel
}

// NewApp creates a new application instance
func NewApp() *App {
	return &App{
		state:        SearchState,
		searchModel:  NewSearchModel(),
		browseModel:  NewBrowseModel(),
		composeModel: NewComposeModel(),
	}
}

// Init initializes the application
func (m *App) Init() tea.Cmd {
	return nil
}

// Update handles messages and updates the application state
func (m *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit

		case "1":
			m.state = SearchState
			return m, nil
		case "2":
			m.state = BrowseState
			return m, nil
		case "3":
			m.state = ComposeState
			return m, nil

		case "tab":
			// Navigate between states
			switch m.state {
			case SearchState:
				m.state = BrowseState
			case BrowseState:
				m.state = ComposeState
			case ComposeState:
				m.state = SearchState
			}
			return m, nil

		case "shift+tab":
			// Navigate backwards between states
			switch m.state {
			case SearchState:
				m.state = ComposeState
			case BrowseState:
				m.state = SearchState
			case ComposeState:
				m.state = BrowseState
			}
			return m, nil
		}
	}

	// Update the current state's model
	switch m.state {
	case SearchState:
		var searchModel tea.Model
		searchModel, cmd = m.searchModel.Update(msg)
		m.searchModel = searchModel.(*SearchModel)
	case BrowseState:
		var browseModel tea.Model
		browseModel, cmd = m.browseModel.Update(msg)
		m.browseModel = browseModel.(*BrowseModel)
	case ComposeState:
		var composeModel tea.Model
		composeModel, cmd = m.composeModel.Update(msg)
		m.composeModel = composeModel.(*ComposeModel)
	}

	return m, cmd
}

// View renders the application
func (m *App) View() string {
	// Header
	header := styles.HeaderStyle.Render("🔍 Prompt Generator")

	// Tab bar
	tabs := m.renderTabs()

	// Main content based on current state
	var content string
	switch m.state {
	case SearchState:
		content = m.searchModel.View()
	case BrowseState:
		content = m.browseModel.View()
	case ComposeState:
		content = m.composeModel.View()
	}

	// Help text
	help := styles.HelpStyle.Render("1,2,3: Jump to tab • Tab/Shift+Tab: Navigate • q/Ctrl+C: Quit")

	// Layout
	main := lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		"",
		tabs,
		"",
		content,
		"",
		help,
	)

	return styles.BaseStyle.
		Width(m.width - 4).
		Height(m.height - 4).
		Render(main)
}

// renderTabs creates the tab bar with colored tabs
func (m *App) renderTabs() string {
	var tabs []string

	// Search tab
	searchIcon := "🔍"
	searchText := " Search "
	if m.state == SearchState {
		tabs = append(tabs, styles.SearchTabStyle.Render(searchIcon+searchText))
	} else {
		tabs = append(tabs, styles.InactiveTabStyle.Render(searchIcon+searchText))
	}

	// Browse tab (show file count if available)
	browseIcon := "📁"
	browseText := " Browse "
	// Add file count indicator
	taggedCount := len(m.browseModel.tagged)
	if taggedCount > 0 {
		browseText = fmt.Sprintf(" Browse (%d) ", taggedCount)
	}
	if m.state == BrowseState {
		tabs = append(tabs, styles.BrowseTabStyle.Render(browseIcon+browseText))
	} else {
		// Show different color if files are tagged
		if taggedCount > 0 {
			taggedStyle := styles.InactiveTabStyle.Copy().
				Foreground(styles.SecondaryColor).
				Bold(true)
			tabs = append(tabs, taggedStyle.Render(browseIcon+browseText))
		} else {
			tabs = append(tabs, styles.InactiveTabStyle.Render(browseIcon+browseText))
		}
	}

	// Compose tab
	composeIcon := "✍️"
	composeText := " Compose "
	if m.state == ComposeState {
		tabs = append(tabs, styles.ComposeTabStyle.Render(composeIcon+composeText))
	} else {
		tabs = append(tabs, styles.InactiveTabStyle.Render(composeIcon+composeText))
	}

	// Join tabs horizontally
	tabBar := lipgloss.JoinHorizontal(lipgloss.Top, tabs...)

	// Add keyboard shortcuts hint
	shortcutHint := styles.HelpStyle.Render("  1,2,3: Jump to tab")
	tabBarWithHint := lipgloss.JoinHorizontal(
		lipgloss.Top,
		tabBar,
		"    ", // Spacer
		shortcutHint,
	)

	return styles.TabBarStyle.Render(tabBarWithHint)
}
