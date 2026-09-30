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

// maxOptionDescriptionRunes は選択肢1行に収まる説明文の目安の長さ。
// huhのMultiSelectはviewportの高さを「1オプション=1行」前提で計算するため
// (m.viewport.Height = len(options))、説明文を丸ごと結合すると折り返して
// 複数行になり、オプション数と実際に見える範囲がずれてスクロールも効かなくなる。
// 1行に収まる長さへ切り詰めることで、この前提を成立させる。
const maxOptionDescriptionRunes = 40

// truncateRunes はsをmax rune以内へ切り詰める。max以下ならそのまま返す。
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
