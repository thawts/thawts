package ai

import (
	"context"
	"strings"
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

func TestStubIsMishap(t *testing.T) {
	p := NewStubProvider()

	cases := []struct {
		input     string
		captureMs int64
		want      bool
	}{
		{"buy milk tomorrow", 0, false},
		{"aX9#kP!2@mZ&qR5s", 0, true},                         // high-entropy, no spaces
		{"const x = () => { return { foo: 'bar' } }", 0, true}, // code snippet
		{"function doThing(a, b) { return a + b; }", 0, true},  // code snippet
		{"normal thought with spaces", 0, false},
		{strings.Repeat("a", 600), 300, true},  // large paste, fast
		{strings.Repeat("a", 600), 2000, false}, // large paste, slow — ok
	}

	for _, tc := range cases {
		got := p.IsMishap(tc.input, tc.captureMs)
		if got != tc.want {
			preview := tc.input
			if len(preview) > 30 {
				preview = preview[:30] + "..."
			}
			t.Errorf("IsMishap(%q, %d) = %v, want %v", preview, tc.captureMs, got, tc.want)
		}
	}
}

func TestStubEmbed(t *testing.T) {
	p := NewStubProvider()
	vec, err := p.Embed(context.Background(), "some text")
	if err != nil {
		t.Fatalf("Embed: unexpected error %v", err)
	}
	if vec != nil {
		t.Errorf("Embed: expected nil from stub, got %v", vec)
	}
}

func TestStubAnalyzeSentiment(t *testing.T) {
	p := NewStubProvider()

	cases := []struct {
		input string
		want  float32 // >0 positive, <0 negative, ==0 neutral
	}{
		{"I am so happy and excited about this great news", 1.0},
		{"this is terrible and awful I hate it", -1.0},
		{"meeting at noon", 0},
		{"", 0},
	}

	for _, tc := range cases {
		score, err := p.AnalyzeSentiment(context.Background(), tc.input)
		if err != nil {
			t.Errorf("AnalyzeSentiment(%q): %v", tc.input, err)
			continue
		}
		switch {
		case tc.want > 0 && score <= 0:
			t.Errorf("AnalyzeSentiment(%q) = %f, want positive", tc.input, score)
		case tc.want < 0 && score >= 0:
			t.Errorf("AnalyzeSentiment(%q) = %f, want negative", tc.input, score)
		case tc.want == 0 && score != 0:
			t.Errorf("AnalyzeSentiment(%q) = %f, want 0", tc.input, score)
		}
	}
}

func TestStubCleanText(t *testing.T) {
	p := NewStubProvider()
	input := "buy mlk tomorow"
	got, err := p.CleanText(context.Background(), input)
	if err != nil {
		t.Fatalf("CleanText: %v", err)
	}
	if got != input {
		t.Errorf("CleanText stub should return input unchanged, got %q", got)
	}
}

func tagNames(tags []ClassifiedTag) []string {
	names := make([]string, len(tags))
	for i, t := range tags {
		names[i] = t.Name
	}
	return names
}
