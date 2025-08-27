package main

import (
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
