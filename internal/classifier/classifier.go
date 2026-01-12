package classifier

import (
	"regexp"
)

// Classifier interface
type Classifier interface {
	Classify(text string) []string
}

// RegexClassifier implements heuristic classification
type RegexClassifier struct {
	rules map[string]*regexp.Regexp
}

// NewRegexClassifier creates a new classifier
func NewRegexClassifier() *RegexClassifier {
	return &RegexClassifier{
		rules: map[string]*regexp.Regexp{
			"TODO":     regexp.MustCompile(`(?i)\b(todo|buy|fix|implement|create|add|update)\b|^\[\s*\]|^\-\s*\[\s*\]`),
			"IDEA":     regexp.MustCompile(`(?i)\b(idea|maybe|think|consider)\b`),
			"QUESTION": regexp.MustCompile(`\?`),
			// Simple calendar keywords for now
			"CALENDAR": regexp.MustCompile(`(?i)\b(tomorrow|yesterday|today|monday|tuesday|wednesday|thursday|friday|saturday|sunday|january|february|march|april|may|june|july|august|september|october|november|december)\b`),
			"QUOTE":    regexp.MustCompile(`^".*"|'s\b`),
		},
	}
}

// Classify returns tags for the text
func (c *RegexClassifier) Classify(text string) []string {
	var tags []string

	for tag, regex := range c.rules {
		if regex.MatchString(text) {
			tags = append(tags, tag)
		}
	}

	// Normalize: If TODO and IDEA overlap, prefer TODO?
	// For now let's keep all potential tags.

	// If no tags and text is short, maybe "NOTE"?
	// Let's keep it clean, only return high confidence matches.

	return tags
}
