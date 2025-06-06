package main

import (
	"log"
	"os" // Added: for file operations
	"prompty/internal/ui/models"

	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	// Open or create a log file. If it already exists, it will be truncated.
	// 0644 means read/write for owner, read-only for others.
	f, err := os.OpenFile("prompty.log", os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		log.Fatalf("Failed to open log file: %v", err)
	}
	defer f.Close() // Ensure the log file is closed when the program exits

	// Set the log output to the file.
	log.SetOutput(f)
	// You can also set a log prefix and flags if desired:
	// log.SetPrefix("prompty: ")
	// log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)

	log.Println("Application started, logging to prompty.log") // Initial log message to confirm setup

	// Initialize the main app model
	m := models.NewApp()

	// Create the Bubble Tea program
	p := tea.NewProgram(m, tea.WithAltScreen())

	// Run the program
	if _, runErr := p.Run(); runErr != nil { // Changed variable name to runErr to avoid shadowing
		log.Fatalf("Bubble Tea program exited with error: %v", runErr) // Log fatal error to file
	}
	log.Println("Application exited cleanly.")
}
