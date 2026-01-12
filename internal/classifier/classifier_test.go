package classifier

import (
	"reflect"
	"sort"
	"testing"
)

func TestRegexClassifier_Classify(t *testing.T) {
	c := NewRegexClassifier()

	tests := []struct {
		text string
		want []string
	}{
		{"Buy milk", []string{"TODO"}},
		{"[ ] Fix bug", []string{"TODO"}},
		{"- [ ] Action item", []string{"TODO"}},
		{"Implement feature", []string{"TODO"}},
		{"I have an idea for a new app", []string{"IDEA"}},
		{"Maybe we should do this", []string{"IDEA"}},
		{"What do you think?", []string{"IDEA", "QUESTION"}},
		{"Meeting tomorrow at 5", []string{"CALENDAR"}},
		{"Lunch on Monday", []string{"CALENDAR"}},
		{"\"Quote of the day\"", []string{"QUOTE"}},
		{"Combine TODO and tomorrow", []string{"TODO", "CALENDAR"}},
		{"No tags here", nil},
	}

	for _, tt := range tests {
		got := c.Classify(tt.text)
		// Sort to ensure deterministic comparison
		sort.Strings(got)
		sort.Strings(tt.want)

		if !reflect.DeepEqual(got, tt.want) {
			t.Errorf("Classify(%q) = %v, want %v", tt.text, got, tt.want)
		}
	}
}
