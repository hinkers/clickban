package ui

import (
	"testing"
	"time"
)

// --- ParseDuration tests ---

func TestParseDuration_HoursAndMinutes(t *testing.T) {
	ms, err := ParseDuration("2h30m")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := int64(2*3600000 + 30*60000)
	if ms != expected {
		t.Errorf("expected %d ms, got %d ms", expected, ms)
	}
}

func TestParseDuration_HoursOnly(t *testing.T) {
	ms, err := ParseDuration("3h")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := int64(3 * 3600000)
	if ms != expected {
		t.Errorf("expected %d ms, got %d ms", expected, ms)
	}
}

func TestParseDuration_MinutesOnly(t *testing.T) {
	ms, err := ParseDuration("45m")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := int64(45 * 60000)
	if ms != expected {
		t.Errorf("expected %d ms, got %d ms", expected, ms)
	}
}

func TestParseDuration_InvalidInput(t *testing.T) {
	_, err := ParseDuration("invalid")
	if err == nil {
		t.Error("expected error for invalid input, got nil")
	}
}

func TestParseDuration_EmptyString(t *testing.T) {
	_, err := ParseDuration("")
	if err == nil {
		t.Error("expected error for empty string, got nil")
	}
}

func TestParseDuration_ZeroHours(t *testing.T) {
	ms, err := ParseDuration("0h30m")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := int64(30 * 60000)
	if ms != expected {
		t.Errorf("expected %d ms, got %d ms", expected, ms)
	}
}

// --- ParseTimeRange tests ---

func TestParseTimeRange_ExplicitTimes(t *testing.T) {
	ref := time.Date(2025, 3, 11, 0, 0, 0, 0, time.Local)
	start, end, err := ParseTimeRange("10:30am to 12:00pm", ref)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if start.Hour() != 10 || start.Minute() != 30 {
		t.Errorf("expected start 10:30, got %02d:%02d", start.Hour(), start.Minute())
	}
	if end.Hour() != 12 || end.Minute() != 0 {
		t.Errorf("expected end 12:00, got %02d:%02d", end.Hour(), end.Minute())
	}
}

func TestParseTimeRange_WithNow(t *testing.T) {
	ref := time.Date(2025, 3, 11, 15, 30, 0, 0, time.Local)
	start, end, err := ParseTimeRange("10:30am to now", ref)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if start.Hour() != 10 || start.Minute() != 30 {
		t.Errorf("expected start 10:30, got %02d:%02d", start.Hour(), start.Minute())
	}
	// end should equal ref
	if !end.Equal(ref) {
		t.Errorf("expected end to equal ref time %v, got %v", ref, end)
	}
}

func TestParseTimeRange_InvalidFormat(t *testing.T) {
	ref := time.Now()
	_, _, err := ParseTimeRange("not a time range", ref)
	if err == nil {
		t.Error("expected error for invalid time range, got nil")
	}
}

func TestParseTimeRange_InvalidStartTime(t *testing.T) {
	ref := time.Now()
	_, _, err := ParseTimeRange("99:99am to 12:00pm", ref)
	if err == nil {
		t.Error("expected error for invalid start time")
	}
}

// --- TimerInput model tests ---

func TestTimerInput_Init(t *testing.T) {
	ti := NewTimerInput()
	view := ti.View()
	if view == "" {
		t.Error("expected non-empty view")
	}
}
