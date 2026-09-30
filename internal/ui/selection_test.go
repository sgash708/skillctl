package ui

import "testing"

func sampleItems() []Item {
	return []Item{
		{ID: "another-skill", Description: "別のskill"},
		{ID: "example-skill", Description: "スリープ抑止"},
	}
}

func TestSelection_ToggleAndSelectedItems(t *testing.T) {
	tests := []struct {
		name                string
		toggleIDs           []string
		expectedSelectedLen int
		expectedID          string
		shouldBeSelected    bool
	}{
		{
			name:                "toggle single item",
			toggleIDs:           []string{"example-skill"},
			expectedSelectedLen: 1,
			expectedID:          "example-skill",
			shouldBeSelected:    true,
		},
		{
			name:                "toggle multiple items",
			toggleIDs:           []string{"example-skill", "another-skill"},
			expectedSelectedLen: 2,
			expectedID:          "example-skill",
			shouldBeSelected:    true,
		},
		{
			name:                "toggle then untoggle",
			toggleIDs:           []string{"example-skill", "example-skill"},
			expectedSelectedLen: 0,
			expectedID:          "example-skill",
			shouldBeSelected:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NewSelection(sampleItems())

			// Initial state should be empty
			if got := s.SelectedItems(); len(got) != 0 {
				t.Fatalf("initial SelectedItems() = %v, want empty", got)
			}

			// Toggle the specified items
			for _, id := range tt.toggleIDs {
				s.Toggle(id)
			}

			// Verify final state
			got := s.SelectedItems()
			if len(got) != tt.expectedSelectedLen {
				t.Errorf("SelectedItems() length = %d, want %d", len(got), tt.expectedSelectedLen)
			}

			if s.IsSelected(tt.expectedID) != tt.shouldBeSelected {
				t.Errorf("IsSelected(%q) = %v, want %v", tt.expectedID, s.IsSelected(tt.expectedID), tt.shouldBeSelected)
			}
		})
	}
}

func TestSelection_Toggle_UnknownID(t *testing.T) {
	tests := []struct {
		name      string
		unknownID string
	}{
		{name: "non-existent ID", unknownID: "does-not-exist"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NewSelection(sampleItems())
			s.Toggle(tt.unknownID)
			if len(s.SelectedItems()) != 0 {
				t.Error("toggling an unknown id must not select anything")
			}
		})
	}
}

func TestTruncateRunes(t *testing.T) {
	tests := []struct {
		name string
		s    string
		max  int
		want string
	}{
		{name: "空文字はそのまま", s: "", max: 10, want: ""},
		{name: "max未満はそのまま", s: "example-skill", max: 40, want: "example-skill"},
		{name: "maxちょうどはそのまま(省略記号を付けない)", s: "0123456789", max: 10, want: "0123456789"},
		{name: "maxを1文字超えたら切り詰めて省略記号を付ける", s: "01234567890", max: 10, want: "0123456789…"},
		{name: "マルチバイト文字はrune単位で数える(バイト単位で壊さない)", s: "あいうえおかきくけこさ", max: 10, want: "あいうえおかきくけこ…"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := truncateRunes(tt.s, tt.max)
			if got != tt.want {
				t.Errorf("truncateRunes(%q, %d) = %q, want %q", tt.s, tt.max, got, tt.want)
			}
		})
	}
}

func TestSelection_SetTarget(t *testing.T) {
	tests := []struct {
		name    string
		target  string
		wantErr bool
	}{
		{name: "claude", target: "claude"},
		{name: "codex", target: "codex"},
		{name: "both", target: "both"},
		{name: "不正な値", target: "invalid", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := NewSelection(sampleItems())
			err := s.SetTarget(tt.target)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if s.Target() != tt.target {
				t.Errorf("Target() = %q, want %q", s.Target(), tt.target)
			}
		})
	}
}
