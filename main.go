package main

import (
	"log"
	"prompty/internal/ui/models" // Corrected import path for models

	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	// Initialize the main app model
	m := models.NewApp()

	// Create the Bubble Tea program.
	// WithAltScreen ensures the app runs in an alternative screen buffer,
	// so the terminal is restored to its original state on exit.
	p := tea.NewProgram(m, tea.WithAltScreen())

	// Run the program. If an error occurs, log it and exit.
	if _, err := p.Run(); err != nil {
		log.Fatal(err)
	}
}
