package ui

import (
	"strings"
	"testing"
)

func TestRenderFooter_ContainsKeyBindings(t *testing.T) {
	bindings := []KeyBinding{
		{Key: "q", Label: "Quit"},
		{Key: "enter", Label: "Select"},
		{Key: "?", Label: "Help"},
	}
	result := RenderFooter(bindings, 80)
	if !strings.Contains(result, "q") {
		t.Errorf("expected footer to contain key 'q', got:\n%s", result)
	}
	if !strings.Contains(result, "Quit") {
		t.Errorf("expected footer to contain label 'Quit', got:\n%s", result)
	}
	if !strings.Contains(result, "enter") {
		t.Errorf("expected footer to contain key 'enter', got:\n%s", result)
	}
	if !strings.Contains(result, "Select") {
		t.Errorf("expected footer to contain label 'Select', got:\n%s", result)
	}
}

func TestRenderFooter_NilBindings(t *testing.T) {
	// Should render without panic even with nil bindings
	result := RenderFooter(nil, 80)
	if result == "" {
		t.Error("expected RenderFooter to return non-empty string even with nil bindings")
	}
}

func TestRenderFooter_EmptyBindings(t *testing.T) {
	result := RenderFooter([]KeyBinding{}, 80)
	if result == "" {
		t.Error("expected RenderFooter to return non-empty string even with empty bindings")
	}
}

func TestRenderFooter_SingleBinding(t *testing.T) {
	bindings := []KeyBinding{
		{Key: "esc", Label: "Back"},
	}
	result := RenderFooter(bindings, 80)
	if !strings.Contains(result, "esc") {
		t.Errorf("expected footer to contain 'esc', got:\n%s", result)
	}
	if !strings.Contains(result, "Back") {
		t.Errorf("expected footer to contain 'Back', got:\n%s", result)
	}
}
