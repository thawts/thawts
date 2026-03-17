package ai

import (
	"testing"
)

func TestStubClassifyThought(t *testing.T) {
	p := NewStubProvider()

	cases := []struct {
		input    string
		wantTags []string
	}{
		{"buy milk", []string{"todo"}},
		{"what if we redesign the homepage?", []string{"idea", "question"}},
		{"meeting tomorrow at 3pm", []string{"calendar"}},
		{"remind me to call dentist", []string{"reminder"}},
		{"\"The best way to predict the future is to invent it\"", []string{"quote"}},
		{"pay the invoice before Friday", []string{"finance", "calendar"}},
		{"", nil},
	}

	for _, tc := range cases {
		c, err := p.ClassifyThought(tc.input)
		if err != nil {
			t.Errorf("ClassifyThought(%q): unexpected error %v", tc.input, err)
			continue
		}
		got := map[string]bool{}
		for _, tag := range c.Tags {
			got[tag.Name] = true
		}
		for _, want := range tc.wantTags {
			if !got[want] {
				t.Errorf("ClassifyThought(%q): missing tag %q (got %v)", tc.input, want, tagNames(c.Tags))
			}
		}
	}
}

func TestStubDetectIntents(t *testing.T) {
	p := NewStubProvider()

	cases := []struct {
		input      string
		wantIntent string
	}{
		{"lunch tomorrow at 12", "calendar"},
		{"remind me to send the report", "reminder"},
		{"- [ ] finish PR review", "task"},
		{"just a normal thought", ""},
	}

	for _, tc := range cases {
		intents, err := p.DetectIntents(tc.input)
		if err != nil {
			t.Errorf("DetectIntents(%q): unexpected error %v", tc.input, err)
			continue
		}
		found := ""
		for _, i := range intents {
			if i.Type == tc.wantIntent {
				found = i.Type
			}
		}
		if tc.wantIntent != "" && found == "" {
			t.Errorf("DetectIntents(%q): want intent %q, got %v", tc.input, tc.wantIntent, intents)
		}
		if tc.wantIntent == "" && len(intents) != 0 {
			t.Errorf("DetectIntents(%q): expected no intents, got %v", tc.input, intents)
		}
	}
}

func tagNames(tags []ClassifiedTag) []string {
	names := make([]string, len(tags))
	for i, t := range tags {
		names[i] = t.Name
	}
	return names
}
