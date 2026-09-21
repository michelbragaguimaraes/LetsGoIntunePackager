package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// typeString feeds s to the model one rune at a time, the way a terminal does.
func typeString(m tea.Model, s string) tea.Model {
	for _, r := range s {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	return m
}

// inputScreen returns a model sitting on the input screen with the source
// folder field focused.
func inputScreen(t *testing.T) tea.Model {
	t.Helper()
	m := tea.Model(NewModel(nil))
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if got := m.(Model).screen; got != ScreenInput {
		t.Fatalf("expected ScreenInput after enter, got %v", got)
	}
	return m
}

func TestTypingPathsContainingQ(t *testing.T) {
	// A bare "q" used to be swallowed by the global quit binding, so these
	// paths could not be entered at all.
	paths := []string{
		`C:\Packages\qBittorrent`,
		`/opt/squid`,
		`C:\q`,
		`qqq`,
	}

	for _, path := range paths {
		t.Run(path, func(t *testing.T) {
			m := typeString(inputScreen(t), path)
			if got := m.(Model).GetSourceFolder(); got != path {
				t.Errorf("source folder = %q, want %q", got, path)
			}
		})
	}
}

func TestQuitKeyDoesNotFireWhileTyping(t *testing.T) {
	_, cmd := inputScreen(t).Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if cmd == nil {
		return
	}
	if _, isQuit := cmd().(tea.QuitMsg); isQuit {
		t.Error(`"q" on the input screen quit the program instead of typing a character`)
	}
}

func TestQuitKeyStillWorksOnWelcome(t *testing.T) {
	m := tea.Model(NewModel(nil))
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if cmd == nil {
		t.Fatal(`"q" on the welcome screen produced no command, want quit`)
	}
	if _, isQuit := cmd().(tea.QuitMsg); !isQuit {
		t.Error(`"q" on the welcome screen did not quit`)
	}
}

func TestCtrlCQuitsWhileTyping(t *testing.T) {
	m := typeString(inputScreen(t), `C:\some\path`)
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd == nil {
		t.Fatal("ctrl+c produced no command, want quit")
	}
	if _, isQuit := cmd().(tea.QuitMsg); !isQuit {
		t.Error("ctrl+c did not quit while a text field had focus")
	}
}

func TestNoKeyQuitsDuringProcessing(t *testing.T) {
	for _, key := range []tea.KeyMsg{
		{Type: tea.KeyRunes, Runes: []rune{'q'}},
		{Type: tea.KeyCtrlC},
	} {
		model := NewModel(nil)
		model.screen = ScreenProcessing
		_, cmd := tea.Model(model).Update(key)
		if cmd == nil {
			continue
		}
		if _, isQuit := cmd().(tea.QuitMsg); isQuit {
			t.Errorf("%v interrupted packaging", key)
		}
	}
}
