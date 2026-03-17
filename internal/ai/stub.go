package ai

import (
	"regexp"
	"strings"
)

// StubProvider is a regex-based AI provider that requires no external dependencies.
// It is designed to be replaced by a real LLM or external API implementation.
type StubProvider struct{}

// NewStubProvider returns a StubProvider.
func NewStubProvider() *StubProvider {
	return &StubProvider{}
}

// ruleSet maps tag names to their detection regexes.
var rules = []struct {
	name    string
	pattern *regexp.Regexp
}{
	{"todo", regexp.MustCompile(`(?i)\b(todo|buy|fix|implement|create|add|update|finish|complete)\b|\[\s*\]`)},
	{"idea", regexp.MustCompile(`(?i)\b(idea|maybe|think|consider|what if|could)\b`)},
	{"question", regexp.MustCompile(`\?`)},
	{"calendar", regexp.MustCompile(`(?i)\b(today|tomorrow|yesterday|monday|tuesday|wednesday|thursday|friday|saturday|sunday|january|february|march|april|may|june|july|august|september|october|november|december|next week|this week)\b`)},
	{"reminder", regexp.MustCompile(`(?i)\b(remind|reminder|don.t forget|remember|alarm)\b`)},
	{"quote", regexp.MustCompile(`"[^"]{5,}"`)},
	{"finance", regexp.MustCompile(`(?i)\b(budget|money|cost|price|pay|invoice|expense|salary|rent|bill)\b|\$\d`)},
}

// ClassifyThought implements Provider.
func (p *StubProvider) ClassifyThought(text string) (*Classification, error) {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return &Classification{}, nil
	}

	var tags []ClassifiedTag
	seen := map[string]bool{}
	for _, rule := range rules {
		if seen[rule.name] {
			continue
		}
		if rule.pattern.MatchString(trimmed) {
			tags = append(tags, ClassifiedTag{Name: rule.name, Confidence: 0.85})
			seen[rule.name] = true
		}
	}
	return &Classification{Tags: tags}, nil
}

// intentPatterns are used for intent detection beyond simple classification.
var intentPatterns = []struct {
	intentType string
	pattern    *regexp.Regexp
}{
	{"calendar", regexp.MustCompile(`(?i)\b(meeting|appointment|lunch|dinner|call|at \d{1,2}(:\d{2})?\s*(am|pm)?)\b`)},
	{"task", regexp.MustCompile(`(?i)^\s*-?\s*\[\s*\]|^\s*(todo|task):`)},
	{"reminder", regexp.MustCompile(`(?i)\b(remind me|don.t forget|remember to)\b`)},
}

// DetectIntents implements Provider.
func (p *StubProvider) DetectIntents(text string) ([]Intent, error) {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return nil, nil
	}

	var intents []Intent
	seen := map[string]bool{}
	for _, ip := range intentPatterns {
		if seen[ip.intentType] {
			continue
		}
		if match := ip.pattern.FindString(trimmed); match != "" {
			intents = append(intents, Intent{
				Type:        ip.intentType,
				Description: trimmed,
				Raw:         match,
			})
			seen[ip.intentType] = true
		}
	}
	return intents, nil
}
