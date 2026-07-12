package memory

import "testing"

func TestCalibrateProviderHitsNormalizesAllProviderDistributions(t *testing.T) {
	t.Parallel()
	tests := map[string][]MemoryHit{
		"sqlite":     {{ID: "s1", Relevance: 1}, {ID: "s2", Relevance: 0.5}},
		"zep":        {{ID: "z1", Relevance: 0.9999}, {ID: "z2", Relevance: 0.9998}},
		"mem0":       {{ID: "m1", Relevance: 0.8}, {ID: "m2", Relevance: 0.4}},
		"mem0-cloud": {{ID: "c1", Relevance: 0.2793}, {ID: "c2", Relevance: 0.21}},
		"jsonrpc":    {{ID: "j1", Relevance: 0.9}, {ID: "j2", Relevance: 0.45}},
	}
	for provider, hits := range tests {
		calibrated := calibrateProviderHits(hits)
		if got, want := calibrated[0].rankingScore, normalizedRelevance(hits[0]); got != want {
			t.Fatalf("%s top calibrated relevance = %f, want %f", provider, got, want)
		}
		if calibrated[0].Relevance != hits[0].Relevance {
			t.Fatalf("%s native relevance changed: %#v", provider, calibrated[0])
		}
		want := normalizedRelevance(hits[1]) / 4
		if got := calibrated[1].rankingScore; absFloat(got-want) > 1e-9 {
			t.Fatalf("%s second calibrated relevance = %f, want %f", provider, got, want)
		}
	}
}

func TestCalibrateProviderHitsKeepsWeakEvidenceWeak(t *testing.T) {
	t.Parallel()
	hits := calibrateProviderHits([]MemoryHit{{ID: "weak-top", Relevance: 0.01}, {ID: "weak-second", Relevance: 0.009}})
	if got, want := hits[0].rankingScore, 0.01; absFloat(got-want) > 1e-9 {
		t.Fatalf("weak top calibrated relevance = %f, want %f", got, want)
	}
	if hits[0].Relevance != 0.01 {
		t.Fatalf("native relevance changed: %#v", hits[0])
	}
}

func TestCalibrateProviderHitsHasNoCandidateCountDiscontinuity(t *testing.T) {
	t.Parallel()
	single := calibrateProviderHits([]MemoryHit{{ID: "top", Relevance: 0.1}})
	multiple := calibrateProviderHits([]MemoryHit{{ID: "top", Relevance: 0.1}, {ID: "tail", Relevance: 0.001}})
	if single[0].rankingScore != multiple[0].rankingScore || single[0].rankingScore != 0.1 {
		t.Fatalf("top changed with candidate count: single=%#v multiple=%#v", single[0], multiple[0])
	}
}

func TestCalibrateProviderHitsPreservesRawScoreAndBoundsCalibration(t *testing.T) {
	t.Parallel()
	raw := 17.5
	hits := calibrateProviderHits([]MemoryHit{
		{ID: "negative", Relevance: -2},
		{ID: "large", Relevance: 4, RawScore: &raw, RawScoreKind: "native"},
	})
	if hits[0].ID != "large" || hits[0].Relevance != 1 || hits[0].rankingScore != 1 || hits[0].RawScore != &raw || hits[0].RawScoreKind != "native" {
		t.Fatalf("calibrated top = %#v", hits[0])
	}
	for _, hit := range hits {
		if hit.rankingScore < 0 || hit.rankingScore > 1 {
			t.Fatalf("out-of-range calibrated relevance: %#v", hit)
		}
	}
}

func TestApplyWeightAndRecencyPreservesWeightOrdering(t *testing.T) {
	t.Parallel()
	weightOne := applyWeightAndRecency(1, 1, 0.2)
	weightTwo := applyWeightAndRecency(1, 2, 0.2)
	weightThree := applyWeightAndRecency(1, 3, 0.2)
	if !(weightOne < weightTwo && weightTwo < weightThree) {
		t.Fatalf("weight ordering lost: %f, %f, %f", weightOne, weightTwo, weightThree)
	}
}

func absFloat(value float64) float64 {
	if value < 0 {
		return -value
	}
	return value
}
