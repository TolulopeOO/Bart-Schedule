package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestInitialModel(t *testing.T) {
	result := initialModel("123456789", []string{"one", "two", "three"})
	if result.message == "" {
		t.Error("expected initial message, got empty string")
	}
	if result.api_key != "123456789" {
		t.Errorf("expected api_key=fake_api_key, got %s", result.api_key)
	}
	if result.cursor != 0 {
		t.Errorf("expected cursor=0, got %d", result.cursor)
	}
}

func TestFetchStations(t *testing.T) {
	mockResponse := apiResponse{
		Root: struct {
			Stations struct {
				Station []station `json:"station"`
			} `json:"stations"`
		}{
			Stations: struct {
				Station []station `json:"station"`
			}{
				Station: []station{
					{Name: "Sample Station A", Abbr: "SamA"},
					{Name: "Sample Station B", Abbr: "SamB"},
				},
			},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(mockResponse)
	}))
	defer server.Close()

	oldGet := httpGet
	httpGet = func(url string) (*http.Response, error) {
		return http.Get(server.URL)
	}
	defer func() { httpGet = oldGet }()

	cmd := fetchStations("fake_key")
	msg := cmd()

	stations, ok := msg.([]station)
	if !ok {
		t.Fatalf("expected []station, got %T", msg)
	}
	if len(stations) != 2 {
		t.Fatalf("expected 2 stations, got %d", len(stations))
	}
	if stations[0].Abbr != "SamA" {
		t.Errorf("expected first station abbr=SamA, got %s", stations[0].Abbr)
	}
}

func TestGetDepartures(t *testing.T) {
	mockResponse := `{
		"root": {
			"station": [{
				"abbr": "SamC",
				"name": "Sample Station C",
				"etd": [{
					"destination": "C",
					"estimate": [{
						"minutes": "5",
						"platform": "1"
					}]
				}]
			}]
		}
	}`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(mockResponse))
	}))
	defer server.Close()

	oldGet := httpGet
	httpGet = func(url string) (*http.Response, error) {
		return http.Get(server.URL)
	}
	defer func() { httpGet = oldGet }()

	deps, err := getDepartures("fake_key", "POWL")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(deps["C"]) == 0 {
		t.Errorf("expected departures for C, got %v", deps)
	}
	if deps["C"][0].Minutes != "5" {
		t.Errorf("expected Minutes=5, got %s", deps["C"][0].Minutes)
	}
}

func TestUpdateQuit(t *testing.T) {
	keys := []string{"q", "Q", "ctrl+c"}

	for _, key := range keys {
		m := model{}
		updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)})

		if cmd == nil {
			t.Errorf("expected Quit command for key %q, got nil", key)
		}
		if updated.(model).cursor != m.cursor {
			t.Errorf("cursor should not change for quit")
		}
	}
}

func TestUpdateMoveCursorUp(t *testing.T) {
	keys := []string{"up", "w", "W"}

	for _, key := range keys {
		m := model{cursor: 1, stations: []station{{}, {}}}
		updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)})

		if updated.(model).cursor != 0 {
			t.Errorf("expected cursor=0 after %q, got %d", key, updated.(model).cursor)
		}
	}
}

func TestUpdateMoveCursorDown(t *testing.T) {
	keys := []string{"down", "s", "S"}

	for _, key := range keys {
		m := model{cursor: 0, stations: []station{{}, {}}}
		updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)})

		if updated.(model).cursor != 1 {
			t.Errorf("expected cursor=1 after %q, got %d", key, updated.(model).cursor)
		}
	}
}

func TestUpdateRefreshStations(t *testing.T) {
	keys := []string{"r", "R"}

	for _, key := range keys {
		m := model{cursor: 2, stations: []station{{}, {}}, info: "old"}
		updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)})

		m2 := updated.(model)

		if m2.cursor != 0 {
			t.Errorf("expected cursor reset to 0, got %d", m2.cursor)
		}
		if m2.stations != nil {
			t.Errorf("expected stations to be cleared, got %v", m2.stations)
		}
		if m2.info != "" {
			t.Errorf("expected info cleared, got %q", m2.info)
		}
		if cmd == nil {
			t.Errorf("expected fetchStations command for key %q, got nil", key)
		}
	}
}

func TestUpdateEnter(t *testing.T) {
	mockResponse := `{
		"root": {
			"station": [{
				"abbr": "SamD",
				"name": "Sample Station D",
				"etd": [{
					"destination": "Dxxx",
					"estimate": [
						{"minutes": "5", "platform": "1"},
						{"minutes": "Leaving", "platform": "2"}
					]
				}]
			}]
		}
	}`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(mockResponse))
	}))
	defer server.Close()

	oldGet := httpGet
	httpGet = func(url string) (*http.Response, error) {
		return http.Get(server.URL)
	}
	defer func() { httpGet = oldGet }()

	m := model{
		api_key:  "fake_key",
		cursor:   0,
		stations: []station{{Name: "Test Station", Abbr: "TEST"}},
	}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m2 := updated.(model)

	if m2.info == "" {
		t.Fatalf("expected departures info, got empty string")
	}
	if !strings.Contains(m2.info, "Dxxx") {
		t.Errorf("expected departures to include Dxxx, got %q", m2.info)
	}
}
