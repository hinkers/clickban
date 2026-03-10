package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func makeTestItems() []PickerItem {
	return []PickerItem{
		{ID: "1", Label: "Alpha"},
		{ID: "2", Label: "Beta"},
		{ID: "3", Label: "Gamma"},
	}
}

func TestPicker_InitialCursorAtZero(t *testing.T) {
	p := NewPicker("Choose", makeTestItems(), false)
	if p.CursorIndex() != 0 {
		t.Errorf("expected initial cursor at 0, got %d", p.CursorIndex())
	}
}

func TestPicker_NavigateDown(t *testing.T) {
	p := NewPicker("Choose", makeTestItems(), false)
	model, _ := p.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	p = model.(Picker)
	if p.CursorIndex() != 1 {
		t.Errorf("expected cursor at 1 after pressing j, got %d", p.CursorIndex())
	}
}

func TestPicker_NavigateUp(t *testing.T) {
	p := NewPicker("Choose", makeTestItems(), false)
	// Move down first
	model, _ := p.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	p = model.(Picker)
	model, _ = p.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	p = model.(Picker)
	// Now move up
	model, _ = p.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'k'}})
	p = model.(Picker)
	if p.CursorIndex() != 1 {
		t.Errorf("expected cursor at 1 after moving down twice then up once, got %d", p.CursorIndex())
	}
}

func TestPicker_NavigateWrapsAtBottom(t *testing.T) {
	p := NewPicker("Choose", makeTestItems(), false)
	// Move down 3 times (wraps back to 0)
	for i := 0; i < 3; i++ {
		model, _ := p.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
		p = model.(Picker)
	}
	if p.CursorIndex() != 0 {
		t.Errorf("expected cursor to wrap to 0, got %d", p.CursorIndex())
	}
}

func TestPicker_SingleSelectConfirm(t *testing.T) {
	p := NewPicker("Choose", makeTestItems(), false)
	// Move to second item
	model, _ := p.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	p = model.(Picker)
	// Confirm with enter
	model, cmd := p.Update(tea.KeyMsg{Type: tea.KeyEnter})
	_ = model
	if cmd == nil {
		t.Error("expected a command after pressing enter")
	}
	// Execute cmd to get result
	msg := cmd()
	result, ok := msg.(PickerResult)
	if !ok {
		t.Fatalf("expected PickerResult, got %T", msg)
	}
	if result.Cancelled {
		t.Error("expected result not to be cancelled")
	}
	if len(result.Selected) != 1 {
		t.Fatalf("expected 1 selected item, got %d", len(result.Selected))
	}
	if result.Selected[0].ID != "2" {
		t.Errorf("expected selected item ID '2', got '%s'", result.Selected[0].ID)
	}
}

func TestPicker_MultiSelectToggle(t *testing.T) {
	p := NewPicker("Choose", makeTestItems(), true)
	// Initially not selected
	if p.IsSelected(0) {
		t.Error("expected item 0 to not be selected initially")
	}
	// Toggle item 0 with space
	model, _ := p.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
	p = model.(Picker)
	if !p.IsSelected(0) {
		t.Error("expected item 0 to be selected after pressing space")
	}
	// Toggle again to deselect
	model, _ = p.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
	p = model.(Picker)
	if p.IsSelected(0) {
		t.Error("expected item 0 to be deselected after pressing space again")
	}
}

func TestPicker_MultiSelectConfirmMultiple(t *testing.T) {
	p := NewPicker("Choose", makeTestItems(), true)
	// Select item 0
	model, _ := p.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
	p = model.(Picker)
	// Move to item 2 and select it
	model, _ = p.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	p = model.(Picker)
	model, _ = p.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	p = model.(Picker)
	model, _ = p.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
	p = model.(Picker)
	// Confirm
	model, cmd := p.Update(tea.KeyMsg{Type: tea.KeyEnter})
	_ = model
	if cmd == nil {
		t.Error("expected a command after pressing enter")
	}
	msg := cmd()
	result, ok := msg.(PickerResult)
	if !ok {
		t.Fatalf("expected PickerResult, got %T", msg)
	}
	if result.Cancelled {
		t.Error("expected result not to be cancelled")
	}
	if len(result.Selected) != 2 {
		t.Errorf("expected 2 selected items, got %d", len(result.Selected))
	}
}

func TestPicker_CancelReturnsCancel(t *testing.T) {
	p := NewPicker("Choose", makeTestItems(), false)
	model, cmd := p.Update(tea.KeyMsg{Type: tea.KeyEsc})
	_ = model
	if cmd == nil {
		t.Error("expected a command after pressing esc")
	}
	msg := cmd()
	result, ok := msg.(PickerResult)
	if !ok {
		t.Fatalf("expected PickerResult, got %T", msg)
	}
	if !result.Cancelled {
		t.Error("expected result to be cancelled")
	}
}

func TestPicker_View(t *testing.T) {
	p := NewPicker("Choose one", makeTestItems(), false)
	view := p.View()
	if view == "" {
		t.Error("expected non-empty view")
	}
}
