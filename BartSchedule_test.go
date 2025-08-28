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
	fakeResp := apiResponse{
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
		json.NewEncoder(w).Encode(fakeResp)
	}))
	defer server.Close()

	origTransport := http.DefaultTransport

	http.DefaultTransport = roundTripperFunc(func(req *http.Request) (*http.Response, error) {
		// Rewrite the request URL host/scheme to match the test server
		req.URL.Scheme = "http"
		req.URL.Host = server.Listener.Addr().String()
		return origTransport.RoundTrip(req)
	})
	defer func() { http.DefaultTransport = origTransport }()

	cmd := fetchStations("fake_api_key")
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

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}
