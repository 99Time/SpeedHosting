package handlers

import (
	"encoding/json"
	"testing"
)

func TestFlexibleStringNumberUnmarshalStringZero(t *testing.T) {
	var request struct {
		GamePlayerNumber flexibleStringNumber `json:"gamePlayerNumber"`
	}

	if err := json.Unmarshal([]byte(`{"gamePlayerNumber":"0"}`), &request); err != nil {
		t.Fatalf("unmarshal string zero: %v", err)
	}

	if !request.GamePlayerNumber.Present {
		t.Fatalf("expected gamePlayerNumber to be present")
	}
	if request.GamePlayerNumber.Value != "0" {
		t.Fatalf("expected value 0, got %q", request.GamePlayerNumber.Value)
	}
}

func TestFlexibleStringNumberUnmarshalNumericZero(t *testing.T) {
	var request struct {
		GamePlayerNumber flexibleStringNumber `json:"gamePlayerNumber"`
	}

	if err := json.Unmarshal([]byte(`{"gamePlayerNumber":0}`), &request); err != nil {
		t.Fatalf("unmarshal numeric zero: %v", err)
	}

	if !request.GamePlayerNumber.Present {
		t.Fatalf("expected gamePlayerNumber to be present")
	}
	if request.GamePlayerNumber.Value != "0" {
		t.Fatalf("expected value 0, got %q", request.GamePlayerNumber.Value)
	}
}
