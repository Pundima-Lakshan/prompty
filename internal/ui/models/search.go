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
	"sort"
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
	resultsViewport viewport.Model  // Added: Viewport for scrollable search results
	allTaggedFiles  []FileItem      // New: Stores all persistently tagged files
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
	// Initial width, will be adjusted by WindowSizeMsg to full available width.
	ti.Width = 50

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
		resultsViewport: vp,           // Initialize the results viewport
		allTaggedFiles:  []FileItem{}, // Initialize the new persistent store
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
// This now returns from the persistent list of all tagged files.
func (m *SearchModel) GetTaggedFiles() []FileItem {
	// Return a copy to prevent external modifications
	copiedFiles := make([]FileItem, len(m.allTaggedFiles))
	copy(copiedFiles, m.allTaggedFiles)
	log.Printf("SearchModel: GetTaggedFiles called, returning %d tagged files from persistent store.", len(copiedFiles))
	return copiedFiles
}

// Update handles messages for the SearchModel.
func (m *SearchModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	var cmd tea.Cmd

	log.Printf("SearchModel Update received message: %T", msg)

	// Handle specific key messages that should bypass textInput/viewport processing
	if kMsg, isKeyMsg := msg.(tea.KeyMsg); isKeyMsg {
		switch kMsg.Type {
		case tea.KeyCtrlA: // Handle Ctrl+A for tagging first, to prevent cursor reset
			log.Printf("SearchModel: Ctrl+A pressed (tag/untag).")
			if m.cursor >= 0 && m.cursor < len(m.results) {
				fileToModify := &m.results[m.cursor]
				fileToModify.Tagged = !fileToModify.Tagged // Toggle status in current results
				log.Printf("SearchModel: Toggled tag for %s. New status in m.results: %v", fileToModify.Path, fileToModify.Tagged)

				// Update m.allTaggedFiles (the persistent store) based on the toggle
				if fileToModify.Tagged {
					// Add to allTaggedFiles if it's not already there
					foundInAllTagged := false
					for _, taggedFile := range m.allTaggedFiles {
						if taggedFile.Path == fileToModify.Path {
							foundInAllTagged = true
							break
						}
					}
					if !foundInAllTagged {
						// Need a deep copy of FileItem if it contains pointers/slices, but for string/bool, direct copy is fine.
						// Ensure content is copied if available to the persistent store.
						if fileToModify.Content == "" { // If content is not loaded yet, schedule it
							cmds = append(cmds, m.loadFileContentCmd(fileToModify.Path))
						}
						m.allTaggedFiles = append(m.allTaggedFiles, *fileToModify)
						log.Printf("SearchModel: Added %s to allTaggedFiles (persistent store).", fileToModify.Path)
					}
				} else {
					// Remove from allTaggedFiles
					newAllTaggedFiles := []FileItem{}
					for _, taggedFile := range m.allTaggedFiles {
						if taggedFile.Path != fileToModify.Path {
							newAllTaggedFiles = append(newAllTaggedFiles, taggedFile)
						}
					}
					m.allTaggedFiles = newAllTaggedFiles
					log.Printf("SearchModel: Removed %s from allTaggedFiles (persistent store).", fileToModify.Path)
				}

				// Always send message to App to update global tagged files
				cmds = append(cmds, func() tea.Msg {
					return TaggedFilesMsg(m.GetTaggedFiles()) // GetTaggedFiles now uses m.allTaggedFiles
				})
			}
			return m, tea.Batch(cmds...) // Return early after handling Ctrl+A
		case tea.KeyCtrlQ:
			log.Printf("SearchModel: Key 'Ctrl+Q' pressed. Quitting application.")
			return m, tea.Quit // Quit the application
		}
	}

	// Now, delegate to text input and viewport for other messages
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

	// Handle other messages
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		log.Printf("SearchModel: WindowSizeMsg received. Width: %d, Height: %d", msg.Width, msg.Height)

		totalAvailableWidth := msg.Width
		// Since there is no preview panel, the left panel can take the full width
		usableContentWidth := totalAvailableWidth

		if usableContentWidth < 0 { // Prevent negative width
			usableContentWidth = 0
		}

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
		m.textInput.Width = usableContentWidth
		m.resultsViewport.Width = usableContentWidth
		m.resultsViewport.Height = availableResultsHeight
		log.Printf("SearchModel: Resized text input to W:%d. Resized results viewport to W:%d H:%d",
			m.textInput.Width, m.resultsViewport.Width, m.resultsViewport.Height)

		// Delegate WindowSizeMsg to text input and viewport
		m.textInput, cmd = m.textInput.Update(msg)
		cmds = append(cmds, cmd)
		m.resultsViewport, cmd = m.resultsViewport.Update(msg)
		cmds = append(cmds, cmd)
		return m, tea.Batch(cmds...)

	case tea.KeyMsg: // Only general key handling, Ctrl+A/Ctrl+Q already handled above
		switch msg.Type {
		case tea.KeyEnter:
			log.Printf("SearchModel: Enter key pressed.")
			// If there's a query, trigger fuzzy search. Otherwise, if query is empty, just show all tagged files.
			if m.textInput.Value() != "" {
				query := m.textInput.Value()
				m.err = nil
				m.querying = true
				// Do NOT clear m.results here directly. The merge logic in FuzzySearchResultsMsg
				// will handle preserving tagged files.
				cmds = append(cmds, runFuzzySearchCmd(query, m.baseDir))
				log.Printf("SearchModel: Triggering fuzzy search on Enter for query: '%s'.", query)
			} else {
				// If query is empty, pressing Enter will display all tagged files.
				m.results = m.GetTaggedFiles()
				m.err = nil
				m.querying = false
				m.cursor = 0
				cmds = append(cmds, func() tea.Msg { return TaggedFilesMsg(m.GetTaggedFiles()) })
				log.Printf("SearchModel: Input empty, showing all tagged files on Enter.")
			}
		case tea.KeyEsc:
			log.Printf("SearchModel: Esc key pressed.")
			// Clear the search query and show all currently tagged files (persistent store)
			m.textInput.SetValue("")
			m.results = m.GetTaggedFiles() // Display only currently tagged files after clearing search
			m.err = nil
			m.querying = false
			m.cursor = 0
			// Reset viewport offset to top when clearing search
			m.resultsViewport.GotoTop()
			cmds = append(cmds, func() tea.Msg { return TaggedFilesMsg(m.GetTaggedFiles()) })
			log.Printf("SearchModel: Search query cleared via Esc. Displaying all tagged files.")
		case tea.KeyCtrlN: // Ctrl+N for navigating down (custom handling)
			log.Printf("SearchModel: Ctrl+N pressed (down).")
			if len(m.results) > 0 {
				m.cursor = (m.cursor + 1) % len(m.results)
				m.resultsViewport.SetYOffset(m.cursor) // Corrected: Use SetYOffset for viewport scrolling
				log.Printf("SearchModel: Cursor moved to %d. Viewport scrolled.", m.cursor)
			}
		case tea.KeyCtrlP: // Ctrl+P for navigating up (custom handling)
			log.Printf("SearchModel: Ctrl+P pressed (up).")
			if len(m.results) > 0 {
				m.cursor = (m.cursor - 1 + len(m.results)) % len(m.results)
				m.resultsViewport.SetYOffset(m.cursor) // Corrected: Use SetYOffset for viewport scrolling
				log.Printf("SearchModel: Cursor moved to %d. Viewport scrolled.", m.cursor)
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
			m.querying = true // Indicate search is active
			// Do NOT clear m.results here directly. The merge logic in FuzzySearchResultsMsg
			// will handle preserving tagged files.
			cmds = append(cmds, runFuzzySearchCmd(query, m.baseDir))
			log.Printf("SearchModel: Debounced fuzzy search triggered for query: '%s'.", query)
		} else {
			log.Printf("SearchModel: Debounced search received, but not enough time passed (%.0fms since last update). Skipping.", time.Since(m.lastUpdate).Milliseconds())
		}
	case FuzzySearchResultsMsg: // Message type for fzf --filter results
		log.Printf("SearchModel: FuzzySearchResultsMsg received. %d paths matched.", len(msg))
		m.querying = false // Fuzzy search is complete

		// Step 1: Initialize displayed results with all currently tagged files.
		// Use a map to efficiently track paths in newCombinedResults and avoid duplicates.
		newCombinedResults := make([]FileItem, 0, len(m.allTaggedFiles)+len(msg))
		seenPathsInCombined := make(map[string]bool)

		for _, item := range m.allTaggedFiles {
			newCombinedResults = append(newCombinedResults, item)
			seenPathsInCombined[item.Path] = true
		}
		log.Printf("SearchModel: Initial combined results with %d allTaggedFiles.", len(newCombinedResults))

		// Step 2: Add new fuzzy search results if not already present (i.e., not a tagged file)
		contentLoadCmds := make([]tea.Cmd, 0)
		for _, p := range msg {
			if !seenPathsInCombined[p] {
				fileItem := FileItem{Path: p, Tagged: false} // Newly found, untagged
				newCombinedResults = append(newCombinedResults, fileItem)
				contentLoadCmds = append(contentLoadCmds, m.loadFileContentCmd(p)) // Schedule content load
				seenPathsInCombined[p] = true                                      // Mark as seen
				log.Printf("SearchModel: Added new fzf result: %s, scheduling content load.", p)
			} else {
				log.Printf("SearchModel: Skipping fzf result %s (already in combined results, likely tagged).", p)
			}
		}

		// Step 3: Sort the combined list by path for consistent display
		sort.Slice(newCombinedResults, func(i, j int) bool {
			return newCombinedResults[i].Path < newCombinedResults[j].Path
		})

		m.results = newCombinedResults // Update the displayed results list

		if len(m.results) == 0 && m.textInput.Value() != "" {
			m.err = fmt.Errorf("no fuzzy matches found for '%s'", m.textInput.Value())
			log.Printf("SearchModel: No fuzzy matches found for query: '%s'.", m.textInput.Value())
		} else {
			m.err = nil
		}

		// Adjust cursor and viewport offset
		if len(m.results) > 0 {
			if m.cursor >= len(m.results) {
				m.cursor = len(m.results) - 1
			} else if m.cursor < 0 {
				m.cursor = 0
			}
			m.resultsViewport.SetYOffset(m.cursor) // Scroll viewport to current cursor
		} else {
			m.cursor = 0
		}
		log.Printf("SearchModel: Updated results with %d combined files. Initiating content loads for new files.", len(m.results))
		cmds = append(cmds, tea.Batch(contentLoadCmds...))
		cmds = append(cmds, func() tea.Msg {
			return TaggedFilesMsg(m.GetTaggedFiles()) // Ensure App model gets updated list
		})
		return m, tea.Batch(cmds...)

	case FuzzySearchErrorMsg:
		log.Printf("SearchModel: FuzzySearchErrorMsg received: %v", msg.Err)
		// On error, show only tagged files if any, otherwise clear results.
		m.results = m.GetTaggedFiles() // Display existing tagged files
		m.err = msg.Err
		m.querying = false
		m.resultsViewport.SetContent("Error: " + msg.Err.Error()) // Show error in viewport
		m.cursor = 0
		cmds = append(cmds, func() tea.Msg { return TaggedFilesMsg(m.GetTaggedFiles()) })
		return m, tea.Batch(cmds...)

	case SearchResultsMsg: // This message type was for previous direct ripgrep output, now mostly unused.
		log.Printf("SearchModel: SearchResultsMsg received (likely from a stale command, not used in fuzzy flow).")
		// This case is largely deprecated as fuzzy search uses FuzzySearchResultsMsg now.
		// If it ever gets triggered, handle it by replacing results and updating tagged.
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
		// On error, show only tagged files if any, otherwise clear results.
		m.results = m.GetTaggedFiles() // Display existing tagged files
		m.err = msg.Err
		m.querying = false
		m.resultsViewport.SetContent("Error: " + msg.Err.Error()) // Show error in viewport
	case fileContentMsg:
		log.Printf("SearchModel: fileContentMsg received for %s. Content length: %d", msg.Path, len(msg.Content))
		// Update content for the file in both m.results (if present) and m.allTaggedFiles
		foundInResults := false
		for i := range m.results {
			if m.results[i].Path == msg.Path {
				m.results[i].Content = msg.Content
				foundInResults = true
				break
			}
		}
		if !foundInResults {
			log.Printf("SearchModel: WARNING - fileContentMsg for %s received, but file not found in current displayed results slice.", msg.Path)
		}

		// Update the content in the persistent store (m.allTaggedFiles)
		for i := range m.allTaggedFiles {
			if m.allTaggedFiles[i].Path == msg.Path {
				m.allTaggedFiles[i].Content = msg.Content
				log.Printf("SearchModel: Content field updated for %s in allTaggedFiles. New length: %d", msg.Path, len(m.allTaggedFiles[i].Content))
				break
			}
		}

		cmds = append(cmds, func() tea.Msg {
			return TaggedFilesMsg(m.GetTaggedFiles()) // Ensure App model gets updated list with content
		})
		log.Printf("SearchModel: Sent TaggedFilesMsg after fileContentMsg (ensuring content is passed).")

	case fileContentErrorMsg:
		log.Printf("SearchModel: fileContentErrorMsg received for %s: %v", msg.Path, msg.Err)
		// No preview pane here to show the error directly to the user for file content.
		// The error is logged. Update the content field to reflect the error if needed
		// in m.results and m.allTaggedFiles to prevent re-attempts for this session.
		for i := range m.results {
			if m.results[i].Path == msg.Path {
				m.results[i].Content = fmt.Sprintf("Error loading content: %v", msg.Err)
				break
			}
		}
		for i := range m.allTaggedFiles {
			if m.allTaggedFiles[i].Path == msg.Path {
				m.allTaggedFiles[i].Content = fmt.Sprintf("Error loading content: %v", msg.Err)
				break
			}
		}
		cmds = append(cmds, func() tea.Msg {
			return TaggedFilesMsg(m.GetTaggedFiles())
		})
	}

	// Any KeyMsg not handled above (Ctrl+A, Ctrl+Q) should be passed to the resultsViewport for scrolling
	// This ensures j/k/ctrl+u/ctrl+d/pageup/pagedown keys work for results scrolling.
	// Note: textinput.Update(msg) has already been called above.
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
		styles.HelpStyle.Render("Type to fuzzy search (auto-updates) • Ctrl+N/Ctrl+P: Navigate • Ctrl+A: Tag/Untag • Esc: Clear Search • Ctrl+Q: Quit • j/k: Scroll Line • Ctrl+U/Ctrl+D: Scroll Half Page • PageUp/PageDown: Scroll Full Page • Mouse Wheel"),
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
		statusSection = lipgloss.NewStyle().
			Foreground(styles.MutedColor).
			Padding(0, 1).
			Render("Start typing to search or press Esc to show all tagged files.")
		resultsContentBuilder.WriteString(statusSection)
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

	// Combine all sections for the main view
	mainView := lipgloss.JoinVertical(
		lipgloss.Left,
		searchSection,
		statusSection,
		resultsSection,
	)

	return mainView
}
