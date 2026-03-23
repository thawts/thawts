//go:build with_onnx

package onnx

import "github.com/thawts/thawts/internal/ai"

// tagExemplars maps each tag to representative sentences in English and German.
// The multilingual model maps both languages into the same vector space, so
// mixed-language centroids work correctly for inputs in either language.
// Embeddings are computed lazily on first ClassifyThought call.
var tagExemplars = map[string][]string{
	"todo": {
		// English
		"buy milk on the way home",
		"fix the login bug before Friday",
		"implement dark mode in the settings",
		"remember to send the invoice",
		"add unit tests for the parser",
		"update the README with new instructions",
		// Deutsch
		"Milch auf dem Heimweg kaufen",
		"den Login-Bug vor Freitag beheben",
		"Dunkelmodus in den Einstellungen implementieren",
		"die Rechnung abschicken nicht vergessen",
		"Unit-Tests für den Parser schreiben",
		"README mit neuen Anweisungen aktualisieren",
	},
	"idea": {
		// English
		"what if we added a search bar to the review screen",
		"consider using Redis for caching",
		"it might be worth trying a different approach here",
		"maybe we could automate this with a script",
		"I wonder if we could simplify the onboarding flow",
		// Deutsch
		"was wäre wenn wir eine Suchleiste hinzufügen",
		"Redis für das Caching könnte sich lohnen",
		"vielleicht sollten wir einen anderen Ansatz ausprobieren",
		"wir könnten das mit einem Skript automatisieren",
		"ich frage mich ob wir das Onboarding vereinfachen könnten",
	},
	"question": {
		// English
		"what does this error message mean",
		"why is the test failing on CI but not locally",
		"how should we handle the edge case here",
		"what is the best way to migrate the database",
		// Deutsch
		"was bedeutet diese Fehlermeldung",
		"warum schlägt der Test in CI fehl aber nicht lokal",
		"wie sollen wir den Grenzfall behandeln",
		"wie migrieren wir die Datenbank am besten",
	},
	"calendar": {
		// English
		"meeting with the team tomorrow at 3pm",
		"dentist appointment on Friday morning",
		"flight to Berlin next Monday",
		"the deadline is end of this month",
		"sync with Sarah on Tuesday at noon",
		// Deutsch
		"Teammeeting morgen um 15 Uhr",
		"Zahnarzttermin am Freitagmorgen",
		"Flug nach Berlin nächsten Montag",
		"Deadline ist Ende dieses Monats",
		"Abstimmung mit Sarah am Dienstag um Mittag",
	},
	"reminder": {
		// English
		"remind me to follow up with the client",
		"don't forget to submit the expense report",
		"remember to water the plants this evening",
		"set an alarm for the standup at 9am",
		// Deutsch
		"daran erinnern beim Kunden nachzufassen",
		"Spesenabrechnung einreichen nicht vergessen",
		"heute Abend die Pflanzen gießen",
		"Wecker für das Standup um 9 Uhr stellen",
	},
	"finance": {
		// English
		"the quarterly budget needs review",
		"invoice due next week for 2400 dollars",
		"rent payment comes out on the first",
		"need to track expenses for the business trip",
		// Deutsch
		"das Quartalsbudget muss überprüft werden",
		"Rechnung über 2400 Euro nächste Woche fällig",
		"Miete wird am Ersten abgebucht",
		"Ausgaben für die Dienstreise erfassen",
	},
}

// centroidThreshold is the minimum cosine similarity for a tag to fire.
const centroidThreshold = 0.42

// classifyWithEmbeddings returns tags by comparing the text embedding against
// pre-computed centroid embeddings for each tag category.
func classifyWithEmbeddings(p *ONNXProvider, text string) (*ai.Classification, error) {
	textEmb, err := p.embed(text)
	if err != nil {
		return nil, err
	}

	if err := p.ensureAnchors(); err != nil {
		return nil, err
	}

	var tags []ai.ClassifiedTag
	for tag, centroid := range p.tagCentroids {
		sim := cosineSim(textEmb, centroid)
		if sim >= centroidThreshold {
			tags = append(tags, ai.ClassifiedTag{Name: tag, Confidence: float64(sim)})
		}
	}
	return &ai.Classification{Tags: tags}, nil
}

// buildTagCentroids embeds all exemplar sentences and averages them per tag.
func buildTagCentroids(p *ONNXProvider) (map[string][]float32, error) {
	centroids := make(map[string][]float32, len(tagExemplars))
	for tag, sentences := range tagExemplars {
		centroid := make([]float32, embeddingDim)
		for _, s := range sentences {
			emb, err := p.embed(s)
			if err != nil {
				return nil, err
			}
			for d := range centroid {
				centroid[d] += emb[d]
			}
		}
		n := float32(len(sentences))
		for d := range centroid {
			centroid[d] /= n
		}
		centroids[tag] = l2Normalize(centroid)
	}
	return centroids, nil
}
