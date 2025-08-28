package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// Allow http.Get to be overridden in tests
var httpGet = http.Get

// Bubbletea model that stores the state of the program
type model struct {
	message      string    //	status message displayed at the top
	stations     []station //	list of all the BART stations
	err          error     //	error state if something fails
	api_key      string    //	API key for the BART API
	cursor       int       //	which station is currently selected on the list
	info         string    //	departure info to be displayed
	args         []string  //	optional CLI arguments
	selectedName string    //	store selected station name for args
}

// Response shape for the BART "stations" API
type apiResponse struct {
	Root struct {
		Stations struct {
			Station []station `json:"station"`
		} `json:"stations"`
	} `json:"root"`
}

// Station object (name, abbreviation, city)
type station struct {
	Name string `json:"name"`
	Abbr string `json:"abbr"`
	City string `json:"city"`
}

// Response shape for the BART "ETD" API (estimated departures)
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

// Simple departure information
type departureInfo struct {
	Minutes  string
	Platform string
}

type tickMsg struct{}

// Creates the initial Bubble Tea model
func initialModel(api_key string, args []string) model {
	return model{
		message: "\nLoading Bart stations...",
		api_key: api_key,
		cursor:  0,
		info:    "",
		args:    args,
	}
}

// Bubble Tea Init: runs once when the program starts
func (m model) Init() tea.Cmd {
	return tea.Batch(
		tea.SetWindowTitle("BART Schedule"),
		fetchStations(m.api_key), //	fetch the station list immediately
		tea.Tick(5*time.Second, func(time.Time) tea.Msg {
			return tickMsg{}
		}),
	)
}

// Fetch the list of all stations
func fetchStations(apiKey string) tea.Cmd {
	return func() tea.Msg {
		url := fmt.Sprintf("https://api.bart.gov/api/stn.aspx?cmd=stns&key=%s&json=y", apiKey)
		resp, err := httpGet(url)
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

		//	Return the stations as a message for Update()
		return data.Root.Stations.Station
	}
}

// Fetch departure times for a given station abbreviation
func getDepartures(apiKey, stationAbbr string) (map[string][]departureInfo, error) {
	url := fmt.Sprintf(
		"https://api.bart.gov/api/etd.aspx?cmd=etd&orig=%s&key=%s&json=y",
		stationAbbr, apiKey,
	)

	resp, err := httpGet(url)
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

	//	If no station data returned, exit early
	if len(data.Root.Station) == 0 {
		return departures, nil
	}

	// Loop through ETD data and collect departures
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

// Handles user input and incoming messages
func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	//	Handles keypresses
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q", "Q":
			return m, tea.Quit
		case "up", "k":
			if m.cursor > 0 {
				m.cursor-- //	Move cursor up
			}
			return m, nil
		case "down", "j":
			if m.cursor < len(m.stations)-1 {
				m.cursor++ //	Move cursor down
			}
			return m, nil
		case "r", "R":
			//	Refresh station list
			m.message = "\nRefreshing stations..."
			m.cursor = 0
			m.stations = nil
			m.info = ""
			return m, fetchStations(m.api_key)
		case "enter":
			//	Show departures for the selected station
			if len(m.stations) > 0 {
				selected := m.stations[m.cursor]
				deps, err := getDepartures(m.api_key, selected.Abbr)
				if err != nil {
					m.info = fmt.Sprintf("Error fetching departures: %v", err)
					return m, nil
				}

				//	Format the departure info
				var infoStr string
				infoStr = selected.Name + "\n\n"
				//	sort the departures in alphabetical order
				var keys []string
				for dest := range deps {
					keys = append(keys, dest)
				}
				sort.Strings(keys)

				for _, dest := range keys {
					depList := deps[dest]
					infoStr += fmt.Sprintf("%s:\n", dest)
					for _, dep := range depList {
						if dep.Minutes == "Leaving" {
							infoStr += fmt.Sprintf(" %s | Platform %s\n", dep.Minutes, dep.Platform)
						} else if min, err := strconv.Atoi(dep.Minutes); err == nil && min < 10 {
							infoStr += fmt.Sprintf("   %s min | Platform %s\n", dep.Minutes, dep.Platform)
						} else {
							infoStr += fmt.Sprintf("  %s min | Platform %s\n", dep.Minutes, dep.Platform)
						}
					}
					infoStr += "\n"
				}

				m.info = infoStr
			}
			return m, nil
		}

	//	Handles message containing stations (from fetchStations)
	case []station:
		m.stations = msg
		m.message = "\nLive Tracking\n============="

		//	If the user provided an argument, skip the list and show departures directly
		if len(m.args) > 0 {
			stationAbbr := strings.ToUpper(m.args[0])
			for _, st := range m.stations {
				if strings.EqualFold(st.Abbr, stationAbbr) {
					//	Save the station name
					m.selectedName = st.Name
					//	fetch departures immediately
					deps, err := getDepartures(m.api_key, st.Abbr)
					if err != nil {
						m.info = fmt.Sprintf("Error fetching departures for %s: %v", st.Abbr, err)
					} else {
						var infoStr string
						infoStr = st.Name + " Departures\n\n"

						//	sort the departures in alphabetical order
						var keys []string
						for dest := range deps {
							keys = append(keys, dest)
						}
						sort.Strings(keys)

						for _, dest := range keys {
							depList := deps[dest]
							infoStr += fmt.Sprintf("%s:\n", dest)
							for _, dep := range depList {
								if dep.Minutes == "Leaving" {
									infoStr += fmt.Sprintf(" %s | Platform %s\n", dep.Minutes, dep.Platform)
								} else if min, err := strconv.Atoi(dep.Minutes); err == nil && min < 10 {
									infoStr += fmt.Sprintf("   %s min | Platform %s\n", dep.Minutes, dep.Platform)
								} else {
									infoStr += fmt.Sprintf("  %s min | Platform %s\n", dep.Minutes, dep.Platform)
								}
							}
							infoStr += "\n"
						}

						m.info = infoStr
					}

					// Clear stations so the station list doesn't render
					m.stations = nil
					break
				}
			}
		}
		return m, nil

	case tickMsg:
		// If locked to a station (args provided), refresh that station’s departures
		if len(m.args) > 0 && m.stations == nil {
			stationAbbr := strings.ToUpper(m.args[0])
			deps, err := getDepartures(m.api_key, stationAbbr)
			if err != nil {
				m.info = fmt.Sprintf("Error refreshing departures for %s: %v", stationAbbr, err)
			} else {
				var infoStr string
				displayName := stationAbbr
				if m.selectedName != "" {
					displayName = m.selectedName
				}
				infoStr = displayName + " Departures\n\n"

				//	sort the departures in alphabetical order
				var keys []string
				for dest := range deps {
					keys = append(keys, dest)
				}
				sort.Strings(keys)

				for _, dest := range keys {
					depList := deps[dest]
					infoStr += fmt.Sprintf("%s:\n", dest)
					for _, dep := range depList {
						if dep.Minutes == "Leaving" {
							infoStr += fmt.Sprintf(" %s | Platform %s\n", dep.Minutes, dep.Platform)
						} else if min, err := strconv.Atoi(dep.Minutes); err == nil && min < 10 {
							infoStr += fmt.Sprintf("   %s min | Platform %s\n", dep.Minutes, dep.Platform)
						} else {
							infoStr += fmt.Sprintf("  %s min | Platform %s\n", dep.Minutes, dep.Platform)
						}
					}
					infoStr += "\n"
				}

				m.info = infoStr
			}
		}

		// schedule the next tick
		return m, tea.Tick(5*time.Second, func(time.Time) tea.Msg {
			return tickMsg{}
		})

	//	Handles errors
	case error:
		m.err = msg
		m.message = "Error loading stations: " + msg.Error()
		return m, nil
	}
	return m, nil
}

// Renders the UI
func (m model) View() string {
	if m.err != nil {
		return fmt.Sprintf("%s\n\nPress 'q' to quit.", m.message)
	}

	// If there is a station list, render side-by-side view
	if len(m.stations) > 0 {

		//	Left side: station list
		stationList := "\nBART Stations:\n\n"

		for i, s := range m.stations {
			cursor := " "
			if i == m.cursor {
				cursor = ">"
			}
			stationList += fmt.Sprintf("%s %s, (%s)\n", cursor, s.Name, s.Abbr)
		}

		//	Right side: departure info (or hint text)
		departures := "\nDepartures:\n\n"
		if m.info != "" {
			departures += m.info
		} else {
			departures += "Press Enter to see departures"
		}

		// Combine left and right columns line by line
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
			out += fmt.Sprintf("%-70s  %s\n", left, right) //	Pad left side to align columns
		}

		return out + "\nPress 'q' to quit. Press 'r' to refresh"
	}

	//	If station list is cleared, show just message + departures
	return fmt.Sprintf("%s\n\n%s\n\nPress 'q' to quit. Press 'r' to refresh", m.message, m.info)
}

func main() {
	api_key := os.Getenv("BART_API_KEY")
	if api_key == "" {
		fmt.Println("\nPlease set BART_API_KEY environment variable: \n\nexport BART_API_KEY=(your api key)\n ")
		os.Exit(1)
	}

	args := os.Args[1:]

	//	Start Bubble Tea program
	//	consider removal of tea.WithAltScreen
	p := tea.NewProgram(initialModel(api_key, args), tea.WithAltScreen())
	if err := p.Start(); err != nil {
		fmt.Printf("\nError starting program: %v\n", err)
		os.Exit(1)
	}
}
