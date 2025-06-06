package search

import (
	"bytes"
	"fmt"
	"os" // Added for os.PathSeparator
	"os/exec"
	"path/filepath" // Added for filepath.Clean
	"strconv"
	"strings"
)

// RipgrepMatch represents a single match from ripgrep.
// It includes the file path, line number, column number, and the matched text itself.
type RipgrepMatch struct {
	File  string // Path to the file where the match was found
	Line  int    // Line number of the match
	Col   int    // Column number (byte offset) of the match on the line
	Text  string // The full line of text containing the match (or a preview if too long)
	Match string // The exact string that matched the pattern
}

// RunRipgrep executes the 'rg' (ripgrep) command with the given pattern and directory.
// It returns a slice of RipgrepMatch objects if successful, or an error otherwise.
// The --vimgrep flag is used to get structured output in the format: file:line:col:text.
func RunRipgrep(pattern string, dir string) ([]RipgrepMatch, error) {
	// Construct the ripgrep command with necessary flags for structured output.
	// -n: show line number
	// -o: show offset (column number)
	// --vimgrep: output in vimgrep format (file:line:col:match_text)
	// --no-messages: suppress ripgrep's informational messages (e.g., binary file warnings)
	// --max-columns-preview: shows a preview of long lines instead of truncating them.
	// --color=never: disables color output to ensure consistent parsing.
	cmd := exec.Command("rg", "-n", "-o", "--vimgrep", "--no-messages", "--max-columns-preview", "--color=never", pattern, dir)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout // Capture standard output
	cmd.Stderr = &stderr // Capture standard error

	err := cmd.Run() // Execute the command
	if err != nil {
		// ripgrep exits with status 1 if no matches are found. This is not an error in our context.
		if exitError, ok := err.(*exec.ExitError); ok && exitError.ExitCode() == 1 {
			// No matches found, return an empty slice of matches and no error.
			return []RipgrepMatch{}, nil
		}
		// For any other error (e.g., ripgrep not found, invalid regex), return a descriptive error.
		return nil, fmt.Errorf("ripgrep command failed: %v\nStderr: %s", err, stderr.String())
	}

	var matches []RipgrepMatch
	// Split the output into individual lines.
	lines := strings.Split(stdout.String(), "\n")
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue // Skip empty lines
		}

		// Parse each line, which should be in the format: {file}:{line}:{col}:{match_text}
		parts := strings.SplitN(line, ":", 4)
		if len(parts) != 4 {
			continue // Skip lines that don't conform to the expected format
		}

		file := parts[0]
		lineNum, err := strconv.Atoi(parts[1]) // Convert line number string to integer
		if err != nil {
			continue
		}
		colNum, err := strconv.Atoi(parts[2]) // Convert column number string to integer
		if err != nil {
			continue
		}
		matchText := parts[3]

		// --- IMPORTANT FIX: Normalize file path ---
		// If the file path returned by ripgrep starts with the base directory
		// (which happens if ripgrep returns absolute paths or paths relative to
		// the root but containing our project root), make it truly relative.
		cleanDir := filepath.Clean(dir)
		if strings.HasPrefix(file, cleanDir) {
			file = strings.TrimPrefix(file, cleanDir)
			// Remove any leading path separator that might remain after trimming the prefix
			file = strings.TrimPrefix(file, string(os.PathSeparator))
		}
		// --- END IMPORTANT FIX ---

		matches = append(matches, RipgrepMatch{
			File:  file,
			Line:  lineNum,
			Col:   colNum,
			Text:  matchText,
			Match: matchText,
		})
	}

	return matches, nil
}
