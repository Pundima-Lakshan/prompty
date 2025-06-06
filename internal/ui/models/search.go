package models

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"prompty/internal/search"
	"prompty/internal/ui/styles"
	"sort"
	"strings"
	"time"

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
	textInput      textinput.Model
	results        []FileItem
	cursor         int
	debounceTicker *time.Ticker
	lastUpdate     time.Time
	querying       bool
	err            error
	baseDir        string
	preview        string
	showPreview    bool
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
	ti.Focus()
	ti.CharLimit = 156
	ti.Width = 50

	baseDir, err := os.Getwd()
	if err != nil {
		log.Printf("SearchModel: Error getting current working directory: %v", err)
	}

	return &SearchModel{
		textInput:      ti,
		results:        []FileItem{},
		cursor:         0,
		debounceTicker: time.NewTicker(300 * time.Millisecond),
		lastUpdate:     time.Now(),
		querying:       false,
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
		log.Printf("searchFilesCmd: Initiating ripgrep for pattern '%s' in dir '%s'", pattern, baseDir)
		matches, err := search.RunRipgrep(pattern, baseDir)
		if err != nil {
			log.Printf("searchFilesCmd: Error running ripgrep: %v", err)
			return SearchErrorMsg{Err: fmt.Errorf("ripgrep search failed: %w", err)}
		}
		log.Printf("searchFilesCmd: Ripgrep returned %d matches.", len(matches))

		// Convert RipgrepMatch to FileItem for uniform handling in results.
		var fileItems []FileItem
		uniqueFiles := make(map[string]struct{})
		for _, m := range matches {
			if _, exists := uniqueFiles[m.File]; !exists {
				fileItems = append(fileItems, FileItem{
					Path:          m.File,
					OriginalMatch: &m,
				})
				uniqueFiles[m.File] = struct{}{}
			}
		}
		log.Printf("searchFilesCmd: Converted to %d unique FileItems.", len(fileItems))

		// Sort file items by path for consistent display.
		sort.Slice(fileItems, func(i, j int) bool {
			return strings.ToLower(fileItems[i].Path) < strings.ToLower(fileItems[j].Path)
		})

		return SearchResultsMsg(fileItems)
	}
}

// readFileContent is a helper function to read the entire content of a file from disk.
func readFileContent(filePath string) (string, error) {
	log.Printf("readFileContent: Attempting to read file: %s", filePath)
	content, err := ioutil.ReadFile(filePath)
	if err != nil {
		log.Printf("readFileContent: Error reading file %s: %v", filePath, err)
		return "", err
	}
	log.Printf("readFileContent: Successfully read %d bytes from %s.", len(content), filePath)
	return string(content), nil
}

// loadFileContentCmd creates a Bubble Tea command to load file content asynchronously.
// This prevents blocking the UI while reading potentially large files.
func (m *SearchModel) loadFileContentCmd(filePath string) tea.Cmd {
	return func() tea.Msg {
		fullPath := filepath.Join(m.baseDir, filePath)
		log.Printf("loadFileContentCmd: Triggered for path: %s (full: %s)", filePath, fullPath)
		content, err := readFileContent(fullPath)
		if err != nil {
			log.Printf("loadFileContentCmd: Error in readFileContent for %s: %v", filePath, err)
			return fileContentErrorMsg{Path: filePath, Err: err}
		}
		log.Printf("loadFileContentCmd: Content loaded for %s.", filePath)
		return fileContentMsg{Path: filePath, Content: content}
	}
}

// GetTaggedFiles returns a slice of FileItem objects that are currently tagged by the user.
func (m *SearchModel) GetTaggedFiles() []FileItem {
	var tagged []FileItem
	for _, file := range m.results {
		if file.Tagged {
			tagged = append(tagged, file)
		}
	}
	log.Printf("SearchModel: GetTaggedFiles called, found %d tagged files.", len(tagged))
	return tagged
}

// Update handles messages for the SearchModel.
func (m *SearchModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	var cmd tea.Cmd

	log.Printf("SearchModel Update received message: %T", msg)

	oldTextInputValue := m.textInput.Value()
	m.textInput, cmd = m.textInput.Update(msg)
	cmds = append(cmds, cmd)

	if m.textInput.Focused() && oldTextInputValue != m.textInput.Value() {
		m.lastUpdate = time.Now()
		if !m.querying {
			cmds = append(cmds, debounceCmd(300*time.Millisecond))
			log.Printf("SearchModel: Text input changed, starting debounce.")
		}
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		log.Printf("SearchModel: KeyMsg received: %s (Type: %d, Mod: %d)", msg.String(), msg.Type)
		switch msg.Type {
		case tea.KeyEnter:
			log.Printf("SearchModel: Enter key pressed.")
			if m.showPreview {
				m.showPreview = false
				m.preview = ""
				log.Printf("SearchModel: Preview closed.")
			} else if m.cursor >= 0 && m.cursor < len(m.results) { // If not in preview, try to open preview
				m.showPreview = true
				m.preview = "Loading content..."
				cmds = append(cmds, m.loadFileContentCmd(m.results[m.cursor].Path))
				log.Printf("SearchModel: Loading content for preview: %s", m.results[m.cursor].Path)
			} else { // No results to preview, trigger search
				query := m.textInput.Value()
				if query != "" {
					m.err = nil
					m.querying = true
					cmds = append(cmds, searchFilesCmd(query, m.baseDir))
					log.Printf("SearchModel: Triggering search for query: %s", query)
				}
			}
		case tea.KeyEsc:
			log.Printf("SearchModel: Esc key pressed.")
			if m.showPreview {
				m.showPreview = false
				m.preview = ""
				log.Printf("SearchModel: Preview closed via Esc.")
			}
		case tea.KeyCtrlN:
			log.Printf("SearchModel: Ctrl+N pressed (down).")
			if len(m.results) > 0 {
				m.cursor = (m.cursor + 1) % len(m.results)
				log.Printf("SearchModel: Cursor moved to %d.", m.cursor)
				if m.showPreview {
					m.preview = "Loading content..."
					cmds = append(cmds, m.loadFileContentCmd(m.results[m.cursor].Path))
					log.Printf("SearchModel: Loading content for preview: %s", m.results[m.cursor].Path)
				}
			}
		case tea.KeyCtrlP:
			log.Printf("SearchModel: Ctrl+P pressed (up).")
			if len(m.results) > 0 {
				m.cursor = (m.cursor - 1 + len(m.results)) % len(m.results)
				log.Printf("SearchModel: Cursor moved to %d.", m.cursor)
				if m.showPreview {
					m.preview = "Loading content..."
					cmds = append(cmds, m.loadFileContentCmd(m.results[m.cursor].Path))
					log.Printf("SearchModel: Loading content for preview: %s", m.results[m.cursor].Path)
				}
			}
		case tea.KeyCtrlA: // Ctrl+A for tagging
			log.Printf("SearchModel: Ctrl+A pressed (tag/untag).")
			if m.cursor >= 0 && m.cursor < len(m.results) {
				// Get a reference to the current file item to modify it in place
				fileToModify := &m.results[m.cursor]

				// Toggle the tagged status
				fileToModify.Tagged = !fileToModify.Tagged
				log.Printf("SearchModel: Toggled tag for %s. New status: %v", fileToModify.Path, fileToModify.Tagged)

				if fileToModify.Tagged { // If the file is now tagged
					if fileToModify.Content == "" { // If content hasn't been loaded yet for this file
						log.Printf("SearchModel: Content not loaded for %s. Initiating load. TaggedFilesMsg will be sent after content arrives.", fileToModify.Path)
						// Initiate content load. TaggedFilesMsg will be sent AFTER content is loaded via fileContentMsg.
						cmds = append(cmds, m.loadFileContentCmd(fileToModify.Path))
					} else {
						// Content is already present (e.g., from a previous preview), send TaggedFilesMsg immediately.
						log.Printf("SearchModel: Content already present for %s. Sending TaggedFilesMsg immediately.", fileToModify.Path)
						cmds = append(cmds, func() tea.Msg {
							return TaggedFilesMsg(m.GetTaggedFiles())
						})
					}
				} else { // If the file is now untagged
					// Untagging does not depend on content loading. Send TaggedFilesMsg immediately.
					log.Printf("SearchModel: Untagged file %s. Sending TaggedFilesMsg immediately.", fileToModify.Path)
					cmds = append(cmds, func() tea.Msg {
						return TaggedFilesMsg(m.GetTaggedFiles())
					})
				}
			}
		}
	case MsgDebouncedSearch:
		log.Printf("SearchModel: MsgDebouncedSearch received.")
		if time.Since(m.lastUpdate) >= 300*time.Millisecond {
			query := m.textInput.Value()
			if query != "" {
				m.err = nil
				m.querying = true
				cmds = append(cmds, searchFilesCmd(query, m.baseDir))
				log.Printf("SearchModel: Debounced search triggered for query: '%s'.", query)
			} else {
				m.results = []FileItem{}
				m.err = nil
				m.querying = false
				log.Printf("SearchModel: Debounced search, query empty, results cleared.")
			}
		} else {
			log.Printf("SearchModel: Debounced search received, but not enough time passed (%.0fms since last update). Skipping.", time.Since(m.lastUpdate).Milliseconds())
		}
	case SearchResultsMsg:
		log.Printf("SearchModel: SearchResultsMsg received. %d results found.", len(msg))
		m.results = msg
		m.querying = false
		m.cursor = 0
		if len(m.results) == 0 && m.textInput.Value() != "" {
			m.err = fmt.Errorf("no matches found for '%s'", m.textInput.Value())
			log.Printf("SearchModel: No matches found for query: '%s'.", m.textInput.Value())
		} else {
			m.err = nil
		}
		// Always inform App model about potentially changed tagged files after search.
		// Existing tags might be on files not returned by new search, or new files can be found.
		cmds = append(cmds, func() tea.Msg {
			return TaggedFilesMsg(m.GetTaggedFiles())
		})
		log.Printf("SearchModel: Sent TaggedFilesMsg after search results update.")
	case SearchErrorMsg:
		log.Printf("SearchModel: SearchErrorMsg received: %v", msg.Err)
		m.results = []FileItem{}
		m.err = msg.Err
		m.querying = false
	case fileContentMsg:
		log.Printf("SearchModel: fileContentMsg received for %s. Content length: %d", msg.Path, len(msg.Content))
		// File content loaded. Update the corresponding FileItem.
		found := false
		for i := range m.results {
			if m.results[i].Path == msg.Path {
				m.results[i].Content = msg.Content
				log.Printf("SearchModel: Content field updated for %s. New length: %d", msg.Path, len(m.results[i].Content))
				found = true
				break
			}
		}
		if !found {
			log.Printf("SearchModel: WARNING - fileContentMsg for %s received, but file not found in current results slice.", msg.Path)
		}

		// If preview is active and the loaded content is for the currently selected file, update preview.
		if m.showPreview && m.cursor >= 0 && m.cursor < len(m.results) && m.results[m.cursor].Path == msg.Path {
			m.preview = msg.Content
			log.Printf("SearchModel: Preview updated for %s.", msg.Path)
		}

		// VERY IMPORTANT: Now that content is loaded, inform the App model with the updated FileItem.
		// This handles the case where the file was newly tagged and its content just arrived.
		cmds = append(cmds, func() tea.Msg {
			return TaggedFilesMsg(m.GetTaggedFiles())
		})
		log.Printf("SearchModel: Sent TaggedFilesMsg after fileContentMsg (ensuring content is passed).")

	case fileContentErrorMsg:
		log.Printf("SearchModel: fileContentErrorMsg received for %s: %v", msg.Path, msg.Err)
		if m.showPreview && m.cursor >= 0 && m.cursor < len(m.results) && m.results[m.cursor].Path == msg.Path {
			m.preview = fmt.Sprintf("Error loading content for %s:\n%s", msg.Path, msg.Err.Error())
		}
		log.Printf("SearchModel: Error displayed in preview for %s.", msg.Path)
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
		resultsSection = lipgloss.NewStyle().
			Foreground(styles.MutedColor).
			Padding(0, 1).
			Render("No files match your search pattern.")
	}

	leftPanel := lipgloss.JoinVertical(
		lipgloss.Left,
		searchSection,
		errorSection,
		resultsSection,
	)

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
