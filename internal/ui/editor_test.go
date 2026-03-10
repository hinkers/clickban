package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestEditor_InitWithContent(t *testing.T) {
	e := NewEditor("Edit name", "initial value")
	if e.Value() != "initial value" {
		t.Errorf("expected initial value 'initial value', got '%s'", e.Value())
	}
}

func TestEditor_ConfirmReturnsResult(t *testing.T) {
	e := NewEditor("Edit name", "hello world")
	model, cmd := e.Update(tea.KeyMsg{Type: tea.KeyEnter})
	_ = model
	if cmd == nil {
		t.Error("expected a command after pressing enter")
	}
	msg := cmd()
	result, ok := msg.(EditorResult)
	if !ok {
		t.Fatalf("expected EditorResult, got %T", msg)
	}
	if result.Cancelled {
		t.Error("expected result not to be cancelled")
	}
	if result.Value != "hello world" {
		t.Errorf("expected value 'hello world', got '%s'", result.Value)
	}
}

func TestEditor_CancelReturnsCancelled(t *testing.T) {
	e := NewEditor("Edit name", "some text")
	model, cmd := e.Update(tea.KeyMsg{Type: tea.KeyEsc})
	_ = model
	if cmd == nil {
		t.Error("expected a command after pressing esc")
	}
	msg := cmd()
	result, ok := msg.(EditorResult)
	if !ok {
		t.Fatalf("expected EditorResult, got %T", msg)
	}
	if !result.Cancelled {
		t.Error("expected result to be cancelled")
	}
}

func TestEditor_View(t *testing.T) {
	e := NewEditor("Edit task name", "current value")
	view := e.View()
	if view == "" {
		t.Error("expected non-empty view")
	}
}
