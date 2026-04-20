package signal_test

import (
	"math"
	"strings"
	"testing"

	"github.com/jonesrussell/north-cloud/infrastructure/signal"
)

func TestEvaluate(t *testing.T) {
	t.Parallel()

	// Representative keyword sets from the two call sites. Kept inline so the
	// test is self-contained and survives either side renaming its lists.
	crawlerPhrases := []string{
		"looking for cto",
		"need a developer",
		"technical co-founder",
		"hiring platform engineer",
		"cloud migration",
		"monolith to microservices",
		"platform modernization",
		"legacy system",
		"technical debt",
		"modernizing stack",
	}
	classifierPhrases := []string{
		"drupal 7",
		"legacy website",
		"site migration",
		"funding announcement",
		"grant funding",
		"hiring",
		"new initiative",
		"platform migration",
		"modernization",
		"wcag compliance",
	}

	tests := []struct {
		name       string
		text       string
		phrases    []string
		wantOK     bool
		wantConf   float64
		wantMinHit int
	}{
		{"empty text", "", crawlerPhrases, false, 0, 0},
		{"empty phrase list", "we need a developer urgently", []string{}, false, 0, 0},
		{"zero matches", "just a regular blog post about nothing", crawlerPhrases, false, 0, 0},

		{"one direct-ask crawler", "we are looking for cto", crawlerPhrases, false, 0, 1},
		{"one weak crawler", "our legacy system needs work", crawlerPhrases, false, 0, 1},
		{"one keyword classifier", "we are hiring someone", classifierPhrases, false, 0, 1},

		{"two crawler keywords", "hiring platform engineer for cloud migration", crawlerPhrases, true, signal.RequiredConfidence, 2},
		{"strong plus weak crawler", "monolith to microservices and technical debt everywhere", crawlerPhrases, true, signal.RequiredConfidence, 2},
		{"two weak crawler", "legacy system plus technical debt everywhere", crawlerPhrases, true, signal.RequiredConfidence, 2},
		{"two classifier keywords", "drupal 7 site migration announced today", classifierPhrases, true, signal.RequiredConfidence, 2},

		{"three crawler keywords", "hiring platform engineer cloud migration legacy system work", crawlerPhrases, true, signal.RequiredConfidence, 2},
		{"four classifier keywords", "drupal 7 legacy website site migration platform migration", classifierPhrases, true, signal.RequiredConfidence, 2},

		{"caller must lowercase", "LOOKING FOR CTO and NEED A DEVELOPER", crawlerPhrases, false, 0, 0},
		{"lowercased caller passes", strings.ToLower("LOOKING FOR CTO and NEED A DEVELOPER"), crawlerPhrases, true, signal.RequiredConfidence, 2},
		{"distinct phrase variants", "we need a developer - looking for cto", crawlerPhrases, true, signal.RequiredConfidence, 2},

		{
			"cross-site parity",
			"hiring platform engineer and modernization work",
			[]string{"hiring platform engineer", "modernization"},
			true, signal.RequiredConfidence, 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			ok, conf, matches := signal.Evaluate(tt.text, tt.phrases)
			if ok != tt.wantOK {
				t.Errorf("ok = %v, want %v", ok, tt.wantOK)
			}
			if math.Abs(conf-tt.wantConf) > 0.0001 {
				t.Errorf("confidence = %v, want %v", conf, tt.wantConf)
			}
			if matches < tt.wantMinHit {
				t.Errorf("matches = %d, want >= %d", matches, tt.wantMinHit)
			}
			if ok && matches < signal.MinKeywordMatches {
				t.Errorf("passing signal must have >= %d matches, got %d",
					signal.MinKeywordMatches, matches)
			}
		})
	}
}

func TestEvaluate_ShortCircuits(t *testing.T) {
	t.Parallel()

	phrases := []string{"alpha", "beta", "gamma", "delta", "epsilon"}
	text := "alpha beta gamma delta epsilon"
	ok, conf, matches := signal.Evaluate(text, phrases)
	if !ok {
		t.Errorf("expected ok=true")
	}
	if math.Abs(conf-signal.RequiredConfidence) > 0.0001 {
		t.Errorf("confidence = %v, want %v", conf, signal.RequiredConfidence)
	}
	if matches != signal.MinKeywordMatches {
		t.Errorf("expected short-circuit at %d, got %d", signal.MinKeywordMatches, matches)
	}
}

func TestConstants(t *testing.T) {
	t.Parallel()

	if signal.MinKeywordMatches != 2 {
		t.Errorf("MinKeywordMatches = %d, want 2 (spec: docs/specs/lead-pipeline.md)", signal.MinKeywordMatches)
	}
	if math.Abs(signal.RequiredConfidence-0.80) > 0.0001 {
		t.Errorf("RequiredConfidence = %v, want 0.80 (spec: docs/specs/lead-pipeline.md)", signal.RequiredConfidence)
	}
}
