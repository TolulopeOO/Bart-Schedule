package main

import (
	"fmt"
	"net/http"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

type model struct {
	message string
}

func initialModel() model {
	return model{
		message: "bart schedule app first test",
	}
}

func (m model) Init() tea.Cmd {
	return tea.SetWindowTitle("BART Schedule")
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q", "Q":
			return m, tea.Quit
		}
	}
	return m, nil
}

func (m model) View() string {
	return fmt.Sprintf("Welcome to the BART Schedule App!\n\nCurrent message: %s\n\nPress 'q' to quit.", m.message)
}

func main() {
	p := tea.NewProgram(initialModel())
	if err := p.Start(); err != nil {
		fmt.Printf("Error starting program: %v\n", err)
		os.Exit(1)
	}

	// Temporary usage to prevent import removal
	_ = fmt.Sprintf
	_ = http.StatusOK
	_ = os.Getenv
	_ = time.Now
	_ = tea.NewProgram
	// -----------
}
