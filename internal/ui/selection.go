package ui

import (
	"fmt"
	"sort"
	"unicode/utf8"
)

type Item struct {
	ID          string
	Description string
}

// maxOptionDescriptionRunes is the target length for a description that fits on one option line.
// huh's MultiSelect computes the viewport height assuming "one option = one line"
// (m.viewport.Height = len(options)), so joining the full description wraps it into
// multiple lines, the option count no longer matches the visible range, and scrolling stops working.
// Truncating to a length that fits on one line makes this assumption hold.
const maxOptionDescriptionRunes = 40

// truncateRunes truncates s to at most max runes. If s is within max, it is returned as is.
func truncateRunes(s string, max int) string {
	if utf8.RuneCountInString(s) <= max {
		return s
	}
	r := []rune(s)
	return string(r[:max]) + "…"
}

type Selection struct {
	items    []Item
	selected map[string]bool
	target   string
}

func NewSelection(items []Item) *Selection {
	return &Selection{items: items, selected: map[string]bool{}}
}

func (s *Selection) Toggle(id string) {
	found := false
	for _, it := range s.items {
		if it.ID == id {
			found = true
			break
		}
	}
	if !found {
		return
	}
	s.selected[id] = !s.selected[id]
}

func (s *Selection) IsSelected(id string) bool {
	return s.selected[id]
}

func (s *Selection) SelectedItems() []Item {
	var out []Item
	for _, it := range s.items {
		if s.selected[it.ID] {
			out = append(out, it)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

func (s *Selection) SetTarget(target string) error {
	switch target {
	case "claude", "codex", "both":
		s.target = target
		return nil
	default:
		return fmt.Errorf("ui: invalid target %q (want claude|codex|both)", target)
	}
}

func (s *Selection) Target() string {
	return s.target
}
