package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
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
				"abbr": "POWL",
				"name": "Powell St.",
				"etd": [{
					"destination": "Dublin",
					"estimate": [{
						"minutes": "5",
						"platform": "2"
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

	if len(deps["Dublin"]) == 0 {
		t.Errorf("expected departures for Dublin, got %v", deps)
	}
	if deps["Dublin"][0].Minutes != "5" {
		t.Errorf("expected Minutes=5, got %s", deps["Dublin"][0].Minutes)
	}
}
