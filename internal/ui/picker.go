// Package ui: this file is a thin adapter for interactive input on a real terminal.
// huh.Form.Run() cannot be tested deterministically without a real terminal,
// so it is explicitly excluded from coverage measurement (the CI threshold check).
// The selection logic itself lives in selection.go and is tested to 100%.
package ui

import (
	"fmt"

	"github.com/charmbracelet/huh"
)

type Picker interface {
	Pick(items []Item) (selectedIDs []string, target string, err error)
}

type HuhPicker struct{}

func (HuhPicker) Pick(items []Item) ([]string, string, error) {
	sel := NewSelection(items)

	options := make([]huh.Option[string], 0, len(items))
	for _, it := range items {
		label := fmt.Sprintf("%s — %s", it.ID, truncateRunes(it.Description, maxOptionDescriptionRunes))
		options = append(options, huh.NewOption(label, it.ID))
	}

	var selectedIDs []string
	var target string

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewMultiSelect[string]().
				Title("Select skills to import").
				Options(options...).
				Value(&selectedIDs),
		),
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Import target").
				Options(
					huh.NewOption("claude", "claude"),
					huh.NewOption("codex", "codex"),
					huh.NewOption("both", "both"),
				).
				Value(&target),
		),
	)

	if err := form.Run(); err != nil {
		return nil, "", fmt.Errorf("ui: picker: %w", err)
	}

	for _, id := range selectedIDs {
		sel.Toggle(id)
	}
	if err := sel.SetTarget(target); err != nil {
		return nil, "", err
	}

	selected := sel.SelectedItems()
	ids := make([]string, 0, len(selected))
	for _, it := range selected {
		ids = append(ids, it.ID)
	}
	return ids, target, nil
}
