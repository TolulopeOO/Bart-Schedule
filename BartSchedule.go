package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

type model struct {
	message  string
	stations []station
	err      error
	api_key  string
}

type apiResponse struct {
	Root struct {
		Stations struct {
			Station []station `json:"station"`
		} `json:"stations"`
	} `json:"root"`
}

type station struct {
	Name string `json:"name"`
	Abbr string `json:"abbr"`
	City string `json:"city"`
}

func initialModel(api_key string) model {
	return model{
		message: "\nLoading Bart stations...",
		api_key: api_key,
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(
		tea.SetWindowTitle("BART Schedule"),
		fetchStations(m.api_key),
	)
}

func fetchStations(apiKey string) tea.Cmd {
	return func() tea.Msg {
		url := fmt.Sprintf("https://api.bart.gov/api/stn.aspx?cmd=stns&key=%s&json=y", apiKey)
		resp, err := http.Get(url)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		var data apiResponse
		if err := json.Unmarshal(body, &data); err != nil {
			return err
		}
		return data.Root.Stations.Station
	}
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q", "Q":
			return m, tea.Quit
		}
	case []station:
		m.stations = msg
		m.message = "Loaded stations!"
		return m, nil
	case error:
		m.err = msg
		m.message = "Error loading stations: " + msg.Error()
		return m, nil
	}
	return m, nil
}

func (m model) View() string {
	if m.err != nil {
		return fmt.Sprintf("%s\n\nPress 'q' to quit.", m.message)
	}
	if len(m.stations) > 0 {
		out := "BART Stations:\n"
		for _, s := range m.stations {
			out += fmt.Sprintf("- %s (%s), %s\n", s.Name, s.Abbr, s.City)
		}
		return out + "\nPress 'q' to quit."
	}
	return fmt.Sprintf("%s\n\nPress 'q' to quit.", m.message)
}

func main() {
	api_key := os.Getenv("BART_API_KEY")
	if api_key == "" {
		fmt.Println("Please set BART_API_KEY environment variable: \n\nexport BART_API_KEY=(your api key)\n")
		os.Exit(1)
	}

	p := tea.NewProgram(initialModel(api_key))
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
