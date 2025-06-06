package main

import (
	"log"
	"prompty/internal/ui/models"

	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	// Initialize the main app model
	m := models.NewApp()

	// Create the Bubble Tea program
	p := tea.NewProgram(m, tea.WithAltScreen())

	// Run the program
	if _, err := p.Run(); err != nil {
		log.Fatal(err)
	}
}
