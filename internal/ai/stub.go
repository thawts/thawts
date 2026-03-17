package ai

import (
	"context"
	"math"
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

// IsMishap implements Provider.
// Returns true for password-like strings, code snippets, or large fast pastes.
func (p *StubProvider) IsMishap(text string, captureMs int64) bool {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return false
	}

	// Large paste: >500 chars entered in under 500ms
	if captureMs > 0 && captureMs < 500 && len(trimmed) > 500 {
		return true
	}

	// Password/token: no spaces, length > 10, 3+ character-type categories
	if !strings.Contains(trimmed, " ") && len(trimmed) > 10 && hasPasswordCharacteristics(trimmed) {
		return true
	}

	// Code snippet: at least 2 code indicators present
	if isCodeSnippet(trimmed) {
		return true
	}

	return false
}

// hasPasswordCharacteristics returns true when text mixes at least 3 of:
// uppercase, lowercase, digits, and special characters.
func hasPasswordCharacteristics(s string) bool {
	var hasUpper, hasLower, hasDigit, hasSpecial bool
	for _, r := range s {
		switch {
		case r >= 'A' && r <= 'Z':
			hasUpper = true
		case r >= 'a' && r <= 'z':
			hasLower = true
		case r >= '0' && r <= '9':
			hasDigit = true
		default:
			hasSpecial = true
		}
	}
	count := 0
	for _, b := range []bool{hasUpper, hasLower, hasDigit, hasSpecial} {
		if b {
			count++
		}
	}
	return count >= 3
}

var codeIndicator = regexp.MustCompile(
	`(?i)\b(function\b|def\s+\w+|import\s+[\w"']|const\s+\w+\s*=|let\s+\w+\s*=|var\s+\w+\s*=|class\s+\w+\b|console\.log|System\.out)\b`,
)

func isCodeSnippet(text string) bool {
	score := 0
	if strings.Contains(text, "{") && strings.Contains(text, "}") {
		score++
	}
	if codeIndicator.MatchString(text) {
		score++
	}
	if strings.Contains(text, "=>") || strings.Contains(text, "->") {
		score++
	}
	if strings.Contains(text, "*/") || strings.Contains(text, "//") {
		score++
	}
	return score >= 2
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

// Embed implements Provider. The stub has no embedding model, so it returns nil.
// Callers must treat nil as "no embedding available" and fall back to text search.
func (p *StubProvider) Embed(_ context.Context, _ string) ([]float32, error) {
	return nil, nil
}

// sentimentWords holds simple positive and negative word lists for VADER-style scoring.
var sentimentWords = struct {
	positive []string
	negative []string
}{
	positive: []string{
		"happy", "good", "great", "love", "wonderful", "excellent", "amazing",
		"joy", "excited", "positive", "brilliant", "beautiful", "success", "win",
		"achieve", "proud", "glad", "calm", "confident", "optimistic", "grateful",
		"thrilled", "fantastic", "awesome", "delighted", "inspired",
	},
	negative: []string{
		"sad", "bad", "awful", "hate", "terrible", "horrible", "depressed",
		"angry", "frustrated", "worry", "stress", "anxious", "fail", "fear",
		"hurt", "problem", "issue", "struggle", "tired", "exhausted", "overwhelmed",
		"disappointed", "miserable", "hopeless", "dread", "pain", "awful", "worst",
	},
}

// AnalyzeSentiment implements Provider using a simple word-list polarity scorer.
// Returns a score in [-1.0, +1.0]: positive = positive sentiment.
func (p *StubProvider) AnalyzeSentiment(_ context.Context, text string) (float32, error) {
	lower := strings.ToLower(text)
	words := strings.Fields(lower)

	var pos, neg int
	for _, w := range words {
		// Strip trailing punctuation for matching
		w = strings.TrimRight(w, ".,!?;:")
		for _, pw := range sentimentWords.positive {
			if w == pw {
				pos++
				break
			}
		}
		for _, nw := range sentimentWords.negative {
			if w == nw {
				neg++
				break
			}
		}
	}
	total := pos + neg
	if total == 0 {
		return 0, nil
	}
	score := float32(pos-neg) / float32(total)
	// Clamp to [-1, +1]
	return float32(math.Max(-1, math.Min(1, float64(score)))), nil
}

// CleanText implements Provider. The stub has no LLM, so it returns the text unchanged.
func (p *StubProvider) CleanText(_ context.Context, text string) (string, error) {
	return text, nil
}
