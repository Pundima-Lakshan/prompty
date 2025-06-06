package models

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"prompty/internal/ui/styles"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// SearchResultsMsg is a custom message type used to carry the results
// back from the asynchronous ripgrep search command. (No longer directly used for main search)
type SearchResultsMsg []FileItem

// SearchErrorMsg is a custom message type to convey errors from the search operation.
type SearchErrorMsg struct {
	Err error
}

// fileContentMsg is a custom message type for when a file's content has been successfully loaded.
type fileContentMsg struct {
	Path    string // Path of the file whose content was loaded
	Content string // The loaded content
}

// fileContentErrorMsg is a custom message type for when an error occurs during file content loading.
type fileContentErrorMsg struct {
	Path string // Path of the file that caused the error
	Err  error  // The error itself
}

// FuzzySearchResultsMsg is a custom message type for results returned by non-interactive fzf.
type FuzzySearchResultsMsg []string // slice of fuzzy-matched file paths

// FuzzySearchErrorMsg is a custom message type for errors from non-interactive fzf.
type FuzzySearchErrorMsg struct {
	Err error
}

// SearchModel handles the search functionality, including the search input,
// displaying results, and allowing navigation and tagging within those results.
type SearchModel struct {
	textInput       textinput.Model // Bubble Tea text input component for search query
	results         []FileItem      // Stores the parsed results as FileItem, allowing tagging
	cursor          int             // Index of the currently highlighted result
	debounceTicker  *time.Ticker    // Ticker for debouncing search queries
	lastUpdate      time.Time       // Timestamp of the last text input update
	querying        bool            // Flag to indicate if a search is in progress (now for fzf execution)
	err             error           // Stores any error that occurred during the search
	baseDir         string          // The base directory for file paths
	preview         string          // Content of the file currently being previewed
	showPreview     bool            // Flag to indicate if the file preview is active
	resultsViewport viewport.Model  // Added: Viewport for scrollable search results
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
	ti.Placeholder = "Type to fuzzy search for files..."
	ti.Focus()
	ti.CharLimit = 156
	ti.Width = 50 // Set width directly during initialization

	// Initialize viewport with arbitrary dimensions, will be updated by WindowSizeMsg
	// Set width based on text input width, initial height arbitrary
	vp := viewport.New(ti.Width, 10)
	vp.HighPerformanceRendering = false // Can set to true for performance, but might redraw more often
	vp.MouseWheelEnabled = true         // Enabled mouse wheel scrolling for search results

	baseDir, err := os.Getwd()
	if err != nil {
		log.Printf("SearchModel: Error getting current working directory: %v", err)
	}

	return &SearchModel{
		textInput:       ti,
		results:         []FileItem{},
		cursor:          0,
		debounceTicker:  time.NewTicker(300 * time.Millisecond),
		lastUpdate:      time.Now(),
		querying:        false,
		err:             nil,
		baseDir:         baseDir,
		preview:         "",
		showPreview:     false,
		resultsViewport: vp, // Initialize the results viewport
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

// getFileListCommand prepares the command to generate the file list.
func getFileListCommand(baseDir string) *exec.Cmd {
	if _, err := exec.LookPath("git"); err == nil {
		gitRevParseCmd := exec.Command("git", "-C", baseDir, "rev-parse", "--is-inside-work-tree")
		var gitRevParseOut bytes.Buffer
		gitRevParseCmd.Stdout = &gitRevParseOut
		if gitRevParseCmd.Run() == nil && strings.TrimSpace(gitRevParseOut.String()) == "true" {
			log.Printf("getFileListCommand: Using 'git ls-files' to get file list.")
			return exec.Command("git", "-C", baseDir, "ls-files")
		}
	}

	log.Printf("getFileListCommand: Using 'rg --files' to get file list.")
	cmd := exec.Command("rg", "--files", "--hidden", "--no-ignore", ".", "--max-depth", "100")
	cmd.Dir = baseDir
	return cmd
}

// runFuzzySearchCmd executes fzf in non-interactive mode to get fuzzy-matched file paths
// by streaming file list to it. This command runs in a goroutine and sends results
// back to the main program loop.
func runFuzzySearchCmd(query string, baseDir string) tea.Cmd {
	return func() tea.Msg { // This function now returns a message when done
		fileListCmd := getFileListCommand(baseDir)
		stdoutPipe, err := fileListCmd.StdoutPipe()
		if err != nil {
			log.Printf("runFuzzySearchCmd (Cmd func): Error creating stdout pipe for file list cmd: %v", err)
			return FuzzySearchErrorMsg{Err: fmt.Errorf("failed to create pipe for file list: %w", err)}
		}
		fileListCmd.Stderr = os.Stderr // Direct file list errors to main stderr for debugging

		fzfArgs := []string{"--filter", query, "--print0"}
		fzfCmd := exec.Command("fzf", fzfArgs...)
		fzfCmd.Stdin = stdoutPipe // Pipe fileListCmd's stdout directly to fzf's stdin

		var stdout, stderr bytes.Buffer
		fzfCmd.Stdout = &stdout
		fzfCmd.Stderr = &stderr

		// Start the file list command
		if err := fileListCmd.Start(); err != nil {
			log.Printf("runFuzzySearchCmd (Cmd func): Error starting file list command: %v", err)
			stdoutPipe.Close() // Close pipe to prevent resource leak
			return FuzzySearchErrorMsg{Err: fmt.Errorf("failed to start file list command: %w", err)}
		}
		log.Printf("runFuzzySearchCmd (Cmd func): Started file list generation for streaming to fzf.")

		// Start the fzf command
		if err := fzfCmd.Start(); err != nil {
			log.Printf("runFuzzySearchCmd (Cmd func): Error starting fzf command: %v", err)
			stdoutPipe.Close()         // Ensure pipe is closed if fzf fails to start
			fileListCmd.Process.Kill() // Try to stop file list command
			fileListCmd.Wait()
			return FuzzySearchErrorMsg{Err: fmt.Errorf("failed to start fzf command: %w", err)}
		}
		log.Printf("runFuzzySearchCmd (Cmd func): Executing fzf --filter with query '%s'", query)

		// Wait for both commands to finish.
		var errs []error

		fzfWaitErr := fzfCmd.Wait()
		if fzfWaitErr != nil {
			errs = append(errs, fmt.Errorf("fzf exited with error: %w (stderr: %s)", fzfWaitErr, stderr.String()))
		}

		fileListWaitErr := fileListCmd.Wait()
		if fileListWaitErr != nil {
			errs = append(errs, fmt.Errorf("file list command exited with error: %w", fileListWaitErr))
		}
		stdoutPipe.Close() // Explicitly close the pipe after both commands are done

		if len(errs) > 0 {
			finalErr := fmt.Errorf("fuzzy search process errors: %v", errs)
			log.Printf("runFuzzySearchCmd (Cmd func): Errors during execution: %v", finalErr)
			return FuzzySearchErrorMsg{Err: finalErr}
		}

		// Parse the null-terminated output from fzf
		rawPaths := bytes.Split(stdout.Bytes(), []byte{0x00})
		var matchedPaths []string
		for _, p := range rawPaths {
			path := string(p)
			if strings.TrimSpace(path) != "" {
				matchedPaths = append(matchedPaths, path)
			}
		}
		log.Printf("runFuzzySearchCmd (Cmd func): fzf --filter returned %d matched paths.", len(matchedPaths))
		return FuzzySearchResultsMsg(matchedPaths) // Send results back to the main Update loop
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
		// IMPORTANT: filePath from fzf should generally be relative to baseDir.
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

	// Always update text input first.
	oldTextInputValue := m.textInput.Value()
	m.textInput, cmd = m.textInput.Update(msg)
	cmds = append(cmds, cmd)

	// If the text input value has changed (meaning a character was typed or deleted),
	// reset the debounce timer and trigger a fuzzy search.
	if m.textInput.Focused() && oldTextInputValue != m.textInput.Value() {
		m.lastUpdate = time.Now()
		// We'll trigger the fuzzy search on debounce.
		if !m.querying {
			cmds = append(cmds, debounceCmd(300*time.Millisecond))
			log.Printf("SearchModel: Text input changed, starting debounce for fuzzy search.")
		}
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		log.Printf("SearchModel: WindowSizeMsg received. Width: %d, Height: %d", msg.Width, msg.Height)

		// Calculate available width for content (adjust for overall BaseStyle padding/borders)
		contentWidth := msg.Width - 4 // Assuming 4 for overall padding/borders

		// Estimate fixed UI height in search tab:
		// Search title: 1 line
		// Search text input: 1 line
		// Spacer: 1 line
		// Help text: 1 line
		// Status section: 1 line
		// Results title: 1 line
		// Spacers: 2 lines
		// Total rough fixed height: 1+1+1+1+1+1+2 = 8 lines
		minFixedUiHeight := 8

		availableResultsHeight := msg.Height - minFixedUiHeight
		if availableResultsHeight < 5 { // Ensure minimum height for results viewport
			availableResultsHeight = 5
		}

		// Update dimensions for text input and results viewport
		m.textInput.Width = contentWidth
		m.resultsViewport.Width = contentWidth
		m.resultsViewport.Height = availableResultsHeight
		log.Printf("SearchModel: Resized text input to W:%d. Resized results viewport to W:%d H:%d",
			m.textInput.Width, m.resultsViewport.Width, m.resultsViewport.Height)

		// Delegate WindowSizeMsg to text input and viewport
		m.textInput, cmd = m.textInput.Update(msg)
		cmds = append(cmds, cmd)
		m.resultsViewport, cmd = m.resultsViewport.Update(msg)
		cmds = append(cmds, cmd)
		return m, tea.Batch(cmds...)

	case tea.KeyMsg:
		log.Printf("SearchModel: KeyMsg received: %s (Type: %d)", msg.String(), msg.Type)
		switch msg.Type {
		case tea.KeyEnter:
			log.Printf("SearchModel: Enter key pressed.")
			if m.showPreview {
				m.showPreview = false
				m.preview = ""
				log.Printf("SearchModel: Preview closed.")
			} else {
				// Enter will now trigger a fuzzy search if not already querying
				query := m.textInput.Value()
				m.err = nil
				m.querying = true
				m.results = []FileItem{} // Clear previous results before new search
				cmds = append(cmds, runFuzzySearchCmd(query, m.baseDir))
				log.Printf("SearchModel: Triggering fuzzy search on Enter for query: '%s'.", query)
			}
		case tea.KeyEsc:
			log.Printf("SearchModel: Esc key pressed.")
			if m.showPreview {
				m.showPreview = false
				m.preview = ""
				log.Printf("SearchModel: Preview closed via Esc.")
			} else {
				// Clear the search query and results
				m.textInput.SetValue("")
				m.results = []FileItem{}
				m.err = nil
				m.querying = false
				m.cursor = 0
				cmds = append(cmds, func() tea.Msg { return TaggedFilesMsg(m.GetTaggedFiles()) })
				log.Printf("SearchModel: Search cleared via Esc.")
			}
		case tea.KeyCtrlN: // Ctrl+N for navigating down (custom handling)
			log.Printf("SearchModel: Ctrl+N pressed (down).")
			if len(m.results) > 0 {
				m.cursor = (m.cursor + 1) % len(m.results)
				m.resultsViewport.SetYOffset(m.cursor) // Corrected: Use SetYOffset for viewport scrolling
				log.Printf("SearchModel: Cursor moved to %d. Viewport scrolled.", m.cursor)
				if m.showPreview {
					m.preview = "Loading content..."
					cmds = append(cmds, m.loadFileContentCmd(m.results[m.cursor].Path))
					log.Printf("SearchModel: Loading content for preview: %s", m.results[m.cursor].Path)
				}
			}
		case tea.KeyCtrlP: // Ctrl+P for navigating up (custom handling)
			log.Printf("SearchModel: Ctrl+P pressed (up).")
			if len(m.results) > 0 {
				m.cursor = (m.cursor - 1 + len(m.results)) % len(m.results)
				m.resultsViewport.SetYOffset(m.cursor) // Corrected: Use SetYOffset for viewport scrolling
				log.Printf("SearchModel: Cursor moved to %d. Viewport scrolled.", m.cursor)
				if m.showPreview {
					m.preview = "Loading content..."
					cmds = append(cmds, m.loadFileContentCmd(m.results[m.cursor].Path))
					log.Printf("SearchModel: Loading content for preview: %s", m.results[m.cursor].Path)
				}
			}
		case tea.KeyCtrlA: // Ctrl+A for tagging (custom handling)
			log.Printf("SearchModel: Ctrl+A pressed (tag/untag).")
			if m.cursor >= 0 && m.cursor < len(m.results) {
				fileToModify := &m.results[m.cursor]
				fileToModify.Tagged = !fileToModify.Tagged
				log.Printf("SearchModel: Toggled tag for %s. New status: %v", fileToModify.Path, fileToModify.Tagged)

				if fileToModify.Tagged {
					if fileToModify.Content == "" {
						log.Printf("SearchModel: Content not loaded for %s. Initiating load. TaggedFilesMsg will be sent after content arrives.", fileToModify.Path)
						cmds = append(cmds, m.loadFileContentCmd(fileToModify.Path))
					} else {
						log.Printf("SearchModel: Content already present for %s. Sending TaggedFilesMsg immediately.", fileToModify.Path)
						cmds = append(cmds, func() tea.Msg {
							return TaggedFilesMsg(m.GetTaggedFiles())
						})
					}
				} else {
					log.Printf("SearchModel: Untagged file %s. Sending TaggedFilesMsg immediately.", fileToModify.Path)
					cmds = append(cmds, func() tea.Msg {
						return TaggedFilesMsg(m.GetTaggedFiles())
					})
				}
			}
		}

	case tea.MouseMsg: // Delegate mouse events to the viewport
		log.Printf("SearchModel: MouseMsg received: %s", msg.String())
		m.resultsViewport, cmd = m.resultsViewport.Update(msg)
		cmds = append(cmds, cmd)

	case MsgDebouncedSearch:
		log.Printf("SearchModel: MsgDebouncedSearch received.")
		if time.Since(m.lastUpdate) >= 300*time.Millisecond {
			query := m.textInput.Value()
			m.err = nil
			m.querying = true        // Indicate search is active
			m.results = []FileItem{} // Clear previous results before new search
			cmds = append(cmds, runFuzzySearchCmd(query, m.baseDir))
			log.Printf("SearchModel: Debounced fuzzy search triggered for query: '%s'.", query)
		} else {
			log.Printf("SearchModel: Debounced search received, but not enough time passed (%.0fms since last update). Skipping.", time.Since(m.lastUpdate).Milliseconds())
		}
	case FuzzySearchResultsMsg: // Message type for fzf --filter results
		log.Printf("SearchModel: FuzzySearchResultsMsg received. %d paths matched.", len(msg))
		m.querying = false // Fuzzy search is complete

		var newResults []FileItem
		contentLoadCmds := make([]tea.Cmd, 0, len(msg))

		// For each matched path, create a FileItem and initiate content loading.
		for _, p := range msg {
			fileItem := FileItem{Path: p, Tagged: false} // Initially not tagged, but matched
			// Check if this file was already in m.results and was tagged.
			// If so, preserve its tagged state and content.
			for _, oldFile := range m.results {
				if oldFile.Path == p {
					fileItem.Tagged = oldFile.Tagged
					fileItem.Content = oldFile.Content // Preserve already loaded content
					break
				}
			}
			newResults = append(newResults, fileItem)
			// Only load content if not already loaded and not already tagged (meaning content is needed for prompt)
			if fileItem.Content == "" {
				contentLoadCmds = append(contentLoadCmds, m.loadFileContentCmd(p))
			}
		}
		m.results = newResults // Update the results list with the new fuzzy matches

		if len(m.results) == 0 && m.textInput.Value() != "" {
			m.err = fmt.Errorf("no fuzzy matches found for '%s'", m.textInput.Value())
			log.Printf("SearchModel: No fuzzy matches found for query: '%s'.", m.textInput.Value())
		} else {
			m.err = nil
		}

		if len(m.results) > 0 {
			m.cursor = 0                // Reset cursor to top for new results
			m.resultsViewport.GotoTop() // Scroll viewport to top
		} else {
			m.cursor = 0
		}
		log.Printf("SearchModel: Updated results with %d fuzzy matches. Initiating content loads for new files.", len(m.results))
		cmds = append(cmds, tea.Batch(contentLoadCmds...))
		cmds = append(cmds, func() tea.Msg {
			return TaggedFilesMsg(m.GetTaggedFiles())
		})
		return m, tea.Batch(cmds...)

	case FuzzySearchErrorMsg:
		log.Printf("SearchModel: FuzzySearchErrorMsg received: %v", msg.Err)
		m.results = []FileItem{}
		m.err = msg.Err
		m.querying = false
		m.resultsViewport.SetContent("Error: " + msg.Err.Error()) // Show error in viewport
		m.cursor = 0
		cmds = append(cmds, func() tea.Msg { return TaggedFilesMsg(m.GetTaggedFiles()) })
		return m, tea.Batch(cmds...)

	case SearchResultsMsg: // This message type was for previous direct ripgrep output, now mostly unused.
		log.Printf("SearchModel: SearchResultsMsg received (likely from a stale command, not used in fuzzy flow).")
		m.results = msg
		m.querying = false
		m.cursor = 0
		if len(m.results) == 0 && m.textInput.Value() != "" {
			m.err = fmt.Errorf("no matches found for '%s'", m.textInput.Value())
		} else {
			m.err = nil
		}
		m.resultsViewport.GotoTop()
		cmds = append(cmds, func() tea.Msg {
			return TaggedFilesMsg(m.GetTaggedFiles())
		})
	case SearchErrorMsg: // This message type is for errors from the underlying ripgrep content search
		log.Printf("SearchModel: SearchErrorMsg received: %v", msg.Err)
		m.results = []FileItem{}
		m.err = msg.Err
		m.querying = false
		m.resultsViewport.SetContent("Error: " + msg.Err.Error()) // Show error in viewport
	case fileContentMsg:
		log.Printf("SearchModel: fileContentMsg received for %s. Content length: %d", msg.Path, len(msg.Content))
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

		if m.showPreview && m.cursor >= 0 && m.cursor < len(m.results) && m.results[m.cursor].Path == msg.Path {
			m.preview = msg.Content
			log.Printf("SearchModel: Preview updated for %s.", msg.Path)
		}

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

	// Any KeyMsg not handled above should be passed to the resultsViewport for scrolling
	// This ensures j/k/ctrl+u/ctrl+d/pageup/pagedown keys work for results scrolling.
	if _, isKeyMsg := msg.(tea.KeyMsg); isKeyMsg {
		m.resultsViewport, cmd = m.resultsViewport.Update(msg)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

// View renders the search interface, including input, results, and optional preview.
func (m *SearchModel) View() string {
	// Search input section
	searchSection := lipgloss.JoinVertical(
		lipgloss.Left,
		lipgloss.NewStyle().Bold(true).Render("🔍 Fuzzy Search Files"),
		"",
		m.textInput.View(),
		"",
		styles.HelpStyle.Render("Type to fuzzy search (auto-updates) • Ctrl+N/Ctrl+P: Navigate • Ctrl+A: Tag/Untag • Enter: Preview Selected • j/k: Scroll Line • Ctrl+U/Ctrl+D: Scroll Half Page • PageUp/PageDown: Scroll Full Page • Mouse Wheel"),
	)

	// Section for displaying any errors or fzf status.
	var statusSection string
	if m.querying {
		statusSection = lipgloss.NewStyle().
			Foreground(styles.MutedColor).
			Padding(0, 1).
			Render("Fuzzy searching and loading content...")
	} else if m.err != nil {
		statusSection = lipgloss.NewStyle().
			Foreground(styles.ErrorColor).
			Padding(0, 1).
			Render(fmt.Sprintf("Error: %s", m.err.Error()))
	} else if len(m.results) == 0 && m.textInput.Value() != "" {
		statusSection = lipgloss.NewStyle().
			Foreground(styles.MutedColor).
			Padding(0, 1).
			Render("No fuzzy matches found for your query.")
	} else if len(m.results) > 0 {
		statusSection = lipgloss.NewStyle().
			Foreground(styles.MutedColor).
			Padding(0, 1).
			Render(fmt.Sprintf("Found %d fuzzy matches.", len(m.results)))
	}

	// Results section
	var resultsSection string
	resultsTitle := lipgloss.NewStyle().Bold(true).Render("📄 Fuzzy Search Results")

	var resultsContentBuilder strings.Builder
	if len(m.results) > 0 {
		for i, fileItem := range m.results {
			var style lipgloss.Style
			cursor := "  "

			// Highlight the currently selected item
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

			// Render the line with its style and append to builder
			resultsContentBuilder.WriteString(style.Render(cursor + tag + fileItem.Path))
			resultsContentBuilder.WriteString("\n")
		}
	} else if m.textInput.Value() == "" && !m.querying && m.err == nil {
		resultsContentBuilder.WriteString(lipgloss.NewStyle().Foreground(styles.MutedColor).Padding(0, 1).Render("Start typing to search..."))
		resultsContentBuilder.WriteString("\n")
	}

	// Set the content to the viewport (always, even if empty to clear previous)
	m.resultsViewport.SetContent(resultsContentBuilder.String())

	resultsSection = lipgloss.JoinVertical(
		lipgloss.Left,
		"",
		resultsTitle,
		"",
		m.resultsViewport.View(), // Render the viewport
	)

	// Combine left panel (search input, status, results)
	leftPanel := lipgloss.JoinVertical(
		lipgloss.Left,
		searchSection,
		statusSection,
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
