package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

type model struct {
	message  string
	stations []station
	err      error
	api_key  string
	cursor   int
	info     string
	args     []string
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

type etdResponse struct {
	Root struct {
		Station []struct {
			Abbr string `json:"abbr"`
			Name string `json:"name"`
			ETD  []struct {
				Destination string `json:"destination"`
				Estimate    []struct {
					Minutes  string `json:"minutes"`
					Platform string `json:"platform"`
				} `json:"estimate"`
			} `json:"etd"`
		} `json:"station"`
	} `json:"root"`
}

type departureInfo struct {
	Minutes  string
	Platform string
}

func initialModel(api_key string, args []string) model {
	return model{
		message: "\nLoading Bart stations...",
		api_key: api_key,
		cursor:  0,
		info:    "",
		args:    args,
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

func getDepartures(apiKey, stationAbbr string) (map[string][]departureInfo, error) {
	url := fmt.Sprintf(
		"https://api.bart.gov/api/etd.aspx?cmd=etd&orig=%s&key=%s&json=y",
		stationAbbr, apiKey,
	)

	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var data etdResponse
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, err
	}

	departures := make(map[string][]departureInfo)

	if len(data.Root.Station) == 0 {
		return departures, nil
	}

	for _, st := range data.Root.Station {
		for _, etd := range st.ETD {
			dest := etd.Destination
			for _, est := range etd.Estimate {
				departures[dest] = append(departures[dest], departureInfo{
					Minutes:  est.Minutes,
					Platform: est.Platform,
				})
			}
		}
	}

	return departures, nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q", "Q":
			return m, tea.Quit
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
			return m, nil
		case "down", "j":
			if m.cursor < len(m.stations)-1 {
				m.cursor++
			}
			return m, nil
		case "r", "R":
			m.message = "\nRefreshing stations..."
			m.cursor = 0
			m.stations = nil
			m.info = ""
			return m, fetchStations(m.api_key)
		case "enter":
			if len(m.stations) > 0 {
				selected := m.stations[m.cursor]
				deps, err := getDepartures(m.api_key, selected.Abbr)
				if err != nil {
					m.info = fmt.Sprintf("Error fetching departures: %v", err)
					return m, nil
				}

				var infoStr string
				infoStr = selected.Name + "\n\n"
				for dest, depList := range deps {
					infoStr += fmt.Sprintf("%s:\n", dest)
					for _, dep := range depList {
						infoStr += fmt.Sprintf("  %s min | Platform %s\n", dep.Minutes, dep.Platform)
					}
					infoStr += "\n"
				}

				m.info = infoStr
			}
			return m, nil
		}
	case []station:
		m.stations = msg
		m.message = "\nLoaded..."
		if len(m.args) > 0 {
			stationAbbr := strings.ToUpper(m.args[0])
			for _, st := range m.stations {
				if strings.EqualFold(st.Abbr, stationAbbr) {
					// Found the station: fetch departures immediately
					deps, err := getDepartures(m.api_key, st.Abbr)
					if err != nil {
						m.info = fmt.Sprintf("Error fetching departures for %s: %v", st.Abbr, err)
					} else {
						var infoStr string
						infoStr = st.Name + " Departures\n\n"
						for dest, depList := range deps {
							infoStr += fmt.Sprintf("%s:\n", dest)
							for _, dep := range depList {
								infoStr += fmt.Sprintf("  %s min | Platform %s\n", dep.Minutes, dep.Platform)
							}
							infoStr += "\n"
						}
						m.info = infoStr
					}

					// Clear stations so we don’t render the picker
					m.stations = nil
					break
				}
			}
		}
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

		stationList := "\nBART Stations:\n\n"

		for i, s := range m.stations {
			cursor := " "
			if i == m.cursor {
				cursor = ">"
			}
			stationList += fmt.Sprintf("%s %s, (%s)\n", cursor, s.Name, s.Abbr)
		}

		departures := "\nDepartures:\n\n"
		if m.info != "" {
			departures += m.info
		} else {
			departures += "Press Enter to see departures"
		}

		leftLines := strings.Split(stationList, "\n")
		rightLines := strings.Split(departures, "\n")

		maxLines := len(leftLines)
		if len(rightLines) > maxLines {
			maxLines = len(rightLines)
		}

		var out string
		for i := 0; i < maxLines; i++ {
			var left, right string
			if i < len(leftLines) {
				left = leftLines[i]
			}
			if i < len(rightLines) {
				right = rightLines[i]
			}
			out += fmt.Sprintf("%-70s  %s\n", left, right)
		}

		return out + "\nPress 'q' to quit. Press 'r' to refresh"
	}

	return fmt.Sprintf("%s\n\n%s\n\nPress 'q' to quit. Press 'r' to refresh", m.message, m.info)
}

func main() {
	api_key := os.Getenv("BART_API_KEY")
	if api_key == "" {
		fmt.Println("\nPlease set BART_API_KEY environment variable: \n\nexport BART_API_KEY=(your api key)\n ")
		os.Exit(1)
	}

	args := os.Args[1:]

	//consider removal of tea.WithAltScreen
	p := tea.NewProgram(initialModel(api_key, args), tea.WithAltScreen())
	if err := p.Start(); err != nil {
		fmt.Printf("\nError starting program: %v\n", err)
		os.Exit(1)
	}
}
