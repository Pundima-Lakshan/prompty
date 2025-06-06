package models

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"prompty/internal/search" // Import the ripgrep functionality
	"prompty/internal/ui/styles"
	"sort"
	"strings"
	"time" // For debouncing

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// SearchResultsMsg is a custom message type used to carry the results
// back from the asynchronous ripgrep search command.
// It now carries a slice of FileItem.
type SearchResultsMsg []FileItem

// SearchErrorMsg is a custom message type to convey errors from the search operation.
type SearchErrorMsg struct {
	Err error
}

// fileContentMsg is a message type for when a file's content has been successfully loaded.
type fileContentMsg struct {
	Path    string // Path of the file whose content was loaded
	Content string // The loaded content
}

// fileContentErrorMsg is a message type for when an error occurs during file content loading.
type fileContentErrorMsg struct {
	Path string // Path of the file that caused the error
	Err  error  // The error itself
}

// SearchModel handles the search functionality, including the search input,
// displaying results, and allowing navigation and tagging within those results.
type SearchModel struct {
	textInput      textinput.Model // Bubble Tea text input component for search query
	results        []FileItem      // Stores the parsed results as FileItem, allowing tagging
	cursor         int             // Index of the currently highlighted result
	debounceTicker *time.Ticker    // Ticker for debouncing search queries
	lastUpdate     time.Time       // Timestamp of the last text input update
	querying       bool            // Flag to indicate if a search is in progress
	err            error           // Stores any error that occurred during the search
	baseDir        string          // The base directory for file paths
	preview        string          // Content of the file currently being previewed
	showPreview    bool            // Flag to indicate if the file preview is active
}

// Init initializes the search model.
// It returns a command to make the text input blink its cursor.
func (m *SearchModel) Init() tea.Cmd {
	return textinput.Blink
}

// NewSearchModel creates and initializes a new SearchModel.
// It sets up the text input, initializes debouncing, and gets the current working directory.
func NewSearchModel() *SearchModel {
	ti := textinput.New()
	ti.Placeholder = "Enter search pattern (e.g., '*.go', 'TODO', 'func main')"
	ti.Focus()         // Focus the text input when the model is created
	ti.CharLimit = 156 // Set a character limit for the input
	ti.Width = 50      // Set the display width of the input

	baseDir, err := os.Getwd()
	if err != nil {
		// Removed log.Printf: log.Printf("Error getting current working directory for search model: %v", err)
	}

	return &SearchModel{
		textInput: ti,
		results:   []FileItem{}, // Initialize with an empty slice of FileItem
		cursor:    0,
		// Debounce ticker: sends a MsgDebouncedSearch after 300ms of inactivity
		debounceTicker: time.NewTicker(300 * time.Millisecond),
		lastUpdate:     time.Now(),
		querying:       false, // Not querying initially
		baseDir:        baseDir,
		preview:        "",
		showPreview:    false,
	}
}

// MsgDebouncedSearch is a custom message sent when the debounce timer finishes.
type MsgDebouncedSearch struct{}

// debounceCmd creates a command that waits for a short period. If it's not
// canceled by another keystroke, it sends a MsgDebouncedSearch.
func debounceCmd(interval time.Duration) tea.Cmd {
	return tea.Tick(interval, func(t time.Time) tea.Msg {
		return MsgDebouncedSearch{}
	})
}

// searchFilesCmd is a Bubble Tea command that asynchronously runs the ripgrep search.
// It takes a search pattern as input.
func searchFilesCmd(pattern string, baseDir string) tea.Cmd {
	return func() tea.Msg {
		// Execute the ripgrep search.
		matches, err := search.RunRipgrep(pattern, baseDir)
		if err != nil {
			// Removed log.Printf: log.Printf("Error running ripgrep: %v", err)
			return SearchErrorMsg{Err: fmt.Errorf("ripgrep search failed: %w", err)}
		}

		// Convert RipgrepMatch to FileItem for uniform handling in results.
		var fileItems []FileItem
		uniqueFiles := make(map[string]struct{}) // Use a map to track unique file paths
		for _, m := range matches {
			if _, exists := uniqueFiles[m.File]; !exists {
				fileItems = append(fileItems, FileItem{
					Path: m.File,
					// Content is loaded lazily when tagged or previewed
					OriginalMatch: &m, // Keep a reference to the original match if needed
				})
				uniqueFiles[m.File] = struct{}{}
			}
		}

		// Sort file items by path for consistent display.
		sort.Slice(fileItems, func(i, j int) bool {
			return strings.ToLower(fileItems[i].Path) < strings.ToLower(fileItems[j].Path)
		})

		return SearchResultsMsg(fileItems)
	}
}

// readFileContent is a helper function to read the entire content of a file from disk.
func readFileContent(filePath string) (string, error) {
	content, err := ioutil.ReadFile(filePath)
	if err != nil {
		return "", err // Return error if file cannot be read
	}
	return string(content), nil // Return content as a string
}

// loadFileContentCmd creates a Bubble Tea command to load file content asynchronously.
// This prevents blocking the UI while reading potentially large files.
func (m *SearchModel) loadFileContentCmd(filePath string) tea.Cmd {
	return func() tea.Msg {
		fullPath := filepath.Join(m.baseDir, filePath) // Construct the full absolute path
		content, err := readFileContent(fullPath)      // Read the file content
		if err != nil {
			// If an error occurs, return a fileContentErrorMsg.
			return fileContentErrorMsg{Path: filePath, Err: err}
		}
		// If successful, return a fileContentMsg with the path and content.
		return fileContentMsg{Path: filePath, Content: content}
	}
}

// GetTaggedFiles returns a slice of FileItem objects that are currently tagged by the user.
func (m *SearchModel) GetTaggedFiles() []FileItem {
	var tagged []FileItem
	for _, file := range m.results {
		if file.Tagged {
			tagged = append(tagged, file) // Include the full FileItem, including content if loaded
		}
	}
	return tagged
}

// Update handles messages for the SearchModel.
func (m *SearchModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd // List of commands to be returned
	var cmd tea.Cmd    // For individual sub-model commands

	// Removed log.Printf: log.Printf("SearchModel Update received message: %T", msg)

	// Always update the text input model first.
	// This ensures that character input is processed and displayed correctly.
	oldTextInputValue := m.textInput.Value()
	m.textInput, cmd = m.textInput.Update(msg)
	cmds = append(cmds, cmd)

	// If the text input value has changed (meaning a character was typed or deleted),
	// reset the debounce timer.
	if m.textInput.Focused() && oldTextInputValue != m.textInput.Value() {
		m.lastUpdate = time.Now()
		// Only start a new debounce command if not already querying
		if !m.querying {
			cmds = append(cmds, debounceCmd(300*time.Millisecond))
			// Removed log.Printf: log.Printf("SearchModel: Text input changed, starting debounce.")
		}
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		// Removed log.Printf: log.Printf("SearchModel: KeyMsg received: %s (Type: %d, Mod: %d)", msg.String(), msg.Type, msg.Mod)
		switch msg.Type { // Changed from msg.String() to msg.Type
		case tea.KeyEnter:
			// Removed log.Printf: log.Printf("SearchModel: Enter key pressed.")
			// If preview is active, close it. Otherwise, trigger search for the input.
			if m.showPreview {
				m.showPreview = false
				m.preview = ""
				// Removed log.Printf: log.Printf("SearchModel: Preview closed.")
			} else { // Trigger search on enter if not in preview
				query := m.textInput.Value()
				if query != "" {
					m.err = nil
					m.querying = true // Set querying flag
					cmds = append(cmds, searchFilesCmd(query, m.baseDir))
					// Removed log.Printf: log.Printf("SearchModel: Triggering search for query: %s", query)
				}
			}
		case tea.KeyEsc:
			// Removed log.Printf: log.Printf("SearchModel: Esc key pressed.")
			// Close preview if active.
			if m.showPreview {
				m.showPreview = false
				m.preview = ""
				// Removed log.Printf: log.Printf("SearchModel: Preview closed via Esc.")
			}
		case tea.KeyCtrlN: // Ctrl+N for navigating down
			// Removed log.Printf: log.Printf("SearchModel: Ctrl+N pressed (down).")
			// Move cursor down.
			if len(m.results) > 0 {
				m.cursor = (m.cursor + 1) % len(m.results)
				// Removed log.Printf: log.Printf("SearchModel: Cursor moved to %d.", m.cursor)
				// If preview is active, update its content.
				if m.showPreview {
					m.preview = "Loading content..."
					cmds = append(cmds, m.loadFileContentCmd(m.results[m.cursor].Path))
					// Removed log.Printf: log.Printf("SearchModel: Loading content for preview: %s", m.results[m.cursor].Path)
				}
			}
		case tea.KeyCtrlP: // Ctrl+P for navigating up
			// Removed log.Printf: log.Printf("SearchModel: Ctrl+P pressed (up).")
			// Move cursor up.
			if len(m.results) > 0 {
				m.cursor = (m.cursor - 1 + len(m.results)) % len(m.results)
				// Removed log.Printf: log.Printf("SearchModel: Cursor moved to %d.", m.cursor)
				// If preview is active, update its content.
				if m.showPreview {
					m.preview = "Loading content..."
					cmds = append(cmds, m.loadFileContentCmd(m.results[m.cursor].Path))
					// Removed log.Printf: log.Printf("SearchModel: Loading content for preview: %s", m.results[m.cursor].Path)
				}
			}
		case tea.KeyCtrlA: // Changed: Now checking for tea.KeyCtrlA for tagging
			// Removed log.Printf: log.Printf("SearchModel: Ctrl+A pressed (tag/untag).")
			// Toggle tag on current file.
			if m.cursor >= 0 && m.cursor < len(m.results) {
				m.results[m.cursor].Tagged = !m.results[m.cursor].Tagged
				// Removed log.Printf: log.Printf("SearchModel: Toggled tag for %s. New status: %v", m.results[m.cursor].Path, m.results[m.cursor].Tagged)

				// If the file is now tagged AND its content hasn't been loaded yet, load it.
				if m.results[m.cursor].Tagged && m.results[m.cursor].Content == "" {
					cmds = append(cmds, m.loadFileContentCmd(m.results[m.cursor].Path))
					// Removed log.Printf: log.Printf("SearchModel: Loading content for tagged file: %s", m.results[m.cursor].Path)
				}

				// Inform the App model that tagged files have changed.
				cmds = append(cmds, func() tea.Msg {
					return TaggedFilesMsg(m.GetTaggedFiles())
				})
				// Removed log.Printf: log.Printf("SearchModel: Sent TaggedFilesMsg.")
			}
		} // end of inner switch tea.KeyMsg
	case MsgDebouncedSearch:
		// Removed log.Printf: log.Printf("SearchModel: MsgDebouncedSearch received.")
		// If debounce finished and enough time has passed since last update, perform search.
		if time.Since(m.lastUpdate) >= 300*time.Millisecond {
			query := m.textInput.Value()
			if query != "" {
				m.err = nil
				m.querying = true // Set querying flag
				cmds = append(cmds, searchFilesCmd(query, m.baseDir))
				// Removed log.Printf: log.Printf("SearchModel: Debounced search triggered for query: %s", query)
			} else {
				m.results = []FileItem{} // Clear results if query is empty
				m.err = nil
				m.querying = false
				// Removed log.Printf: log.Printf("SearchModel: Debounced search, query empty, results cleared.")
			}
		} else {
			// Removed log.Printf: log.Printf("SearchModel: Debounced search received, but not enough time passed. Skipping.")
		}
	case SearchResultsMsg:
		// Removed log.Printf: log.Printf("SearchModel: SearchResultsMsg received. %d results found.", len(msg))
		// Search results received.
		m.results = msg    // Update results with FileItems
		m.querying = false // Clear querying flag
		m.cursor = 0       // Reset cursor to top
		if len(m.results) == 0 && m.textInput.Value() != "" {
			m.err = fmt.Errorf("no matches found for '%s'", m.textInput.Value())
			// Removed log.Printf: log.Printf("SearchModel: No matches found for query: %s", m.textInput.Value())
		} else {
			m.err = nil
		}
		// Inform App model about potentially changed tagged files.
		cmds = append(cmds, func() tea.Msg {
			return TaggedFilesMsg(m.GetTaggedFiles())
		})
		// Removed log.Printf: log.Printf("SearchModel: Sent TaggedFilesMsg after search results update.")
	case SearchErrorMsg:
		// Removed log.Printf: log.Printf("SearchModel: SearchErrorMsg received: %v", msg.Err)
		// Search error received.
		m.results = []FileItem{} // Clear results on error
		m.err = msg.Err          // Store the error
		m.querying = false       // Clear querying flag
	case fileContentMsg:
		// Removed log.Printf: log.Printf("SearchModel: fileContentMsg received for %s.", msg.Path)
		// File content loaded. Update the corresponding FileItem and preview if active.
		for i := range m.results {
			if m.results[i].Path == msg.Path {
				m.results[i].Content = msg.Content
				// Removed log.Printf: log.Printf("SearchModel: Content loaded for %s.", msg.Path)
				break
			}
		}
		if m.showPreview && m.cursor >= 0 && m.cursor < len(m.results) && m.results[m.cursor].Path == msg.Path {
			m.preview = msg.Content
			// Removed log.Printf: log.Printf("SearchModel: Preview updated for %s.", msg.Path)
		}
		// Also inform App model about updated tagged files.
		cmds = append(cmds, func() tea.Msg {
			return TaggedFilesMsg(m.GetTaggedFiles())
		})
		// Removed log.Printf: log.Printf("SearchModel: Sent TaggedFilesMsg after file content update.")
	case fileContentErrorMsg:
		// Removed log.Printf: log.Printf("SearchModel: fileContentErrorMsg received for %s: %v", msg.Path, msg.Err)
		// Error loading file content. Display in preview if active.
		if m.showPreview && m.cursor >= 0 && m.cursor < len(m.results) && m.results[m.cursor].Path == msg.Path {
			m.preview = fmt.Sprintf("Error loading content for %s:\n%s", msg.Path, msg.Err.Error())
		}
		// Removed log.Printf: log.Printf("SearchModel: Error in preview for %s.", msg.Path)
	}

	return m, tea.Batch(cmds...)
}

// View renders the search interface, including input, results, and optional preview.
func (m *SearchModel) View() string {
	// Search input section
	searchSection := lipgloss.JoinVertical(
		lipgloss.Left,
		lipgloss.NewStyle().Bold(true).Render("🔍 Search Files (Real-time)"),
		"",
		m.textInput.View(),
		"",
		// Updated help text for new keybindings
		styles.HelpStyle.Render("Type to search • Ctrl+N/Ctrl+P: Navigate • Ctrl+A: Tag/Untag • Enter: Preview"),
	)

	// Section for displaying any errors.
	var errorSection string
	if m.err != nil {
		errorSection = lipgloss.NewStyle().
			Foreground(styles.ErrorColor).
			Padding(0, 1).
			Render(fmt.Sprintf("Error: %s", m.err.Error()))
	} else if m.querying {
		// Show a loading indicator when a search is in progress
		errorSection = lipgloss.NewStyle().
			Foreground(styles.MutedColor).
			Padding(0, 1).
			Render("Searching...")
	}

	// Results section
	var resultsSection string
	if len(m.results) > 0 {
		resultsTitle := lipgloss.NewStyle().Bold(true).Render("📄 Search Results")

		var resultsList []string
		for i, fileItem := range m.results {
			var style lipgloss.Style
			cursor := "  "

			if i == m.cursor {
				cursor = "▶ "
				if fileItem.Tagged {
					style = styles.TaggedStyle
				} else {
					style = styles.SelectedStyle
				}
			} else if fileItem.Tagged {
				style = styles.TaggedStyle
			} else {
				style = styles.NormalStyle
			}

			tag := "  "
			if fileItem.Tagged {
				tag = "✓ "
			}

			line := cursor + tag + fileItem.Path
			resultsList = append(resultsList, style.Render(line))
		}

		resultsContent := lipgloss.JoinVertical(lipgloss.Left, resultsList...)

		resultsSection = lipgloss.JoinVertical(
			lipgloss.Left,
			"",
			resultsTitle,
			"",
			resultsContent,
		)
	} else if m.textInput.Value() != "" && m.err == nil && !m.querying {
		// Only show this if there's a query but no results and no error, and not currently querying
		resultsSection = lipgloss.NewStyle().
			Foreground(styles.MutedColor).
			Padding(0, 1).
			Render("No files match your search pattern.")
	}

	// Combine left panel (search input, errors, results)
	leftPanel := lipgloss.JoinVertical(
		lipgloss.Left,
		searchSection,
		errorSection,
		resultsSection,
	)

	// If preview is shown, create two-column layout
	if m.showPreview && m.preview != "" {
		previewTitle := lipgloss.NewStyle().Bold(true).Render(
			fmt.Sprintf("👁 Preview: %s", m.results[m.cursor].Path),
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
