package memory

import "sort"

// calibrateProviderHits derives a provider-independent ranking signal while
// preserving the adapter's relevance and raw score. Calibration deliberately
// uses squared reciprocal rank to regularize each provider's normalized evidence.
// Native score magnitudes are not treated as calibrated probabilities, and no
// candidate is promoted above the evidence supplied by its adapter.
func calibrateProviderHits(hits []MemoryHit) []MemoryHit {
	calibrated := append([]MemoryHit(nil), hits...)
	sort.SliceStable(calibrated, func(i, j int) bool {
		return normalizedRelevance(calibrated[i]) > normalizedRelevance(calibrated[j])
	})
	if len(calibrated) == 0 {
		return calibrated
	}

	for i := range calibrated {
		native := normalizedRelevance(calibrated[i])
		calibrated[i].Relevance = native
		if native <= 0 {
			calibrated[i].rankingScore = 0
			continue
		}
		rank := float64(i + 1)
		rankPrior := 1 / (rank * rank)
		calibrated[i].rankingScore = native * rankPrior
	}
	return calibrated
}

func applyWeightAndRecency(relevance, weight, recency float64) float64 {
	if weight <= 0 {
		weight = 1
	}
	return clampUnit(relevance)*weight + clampUnit(recency)
}

func clampUnit(value float64) float64 {
	if value < 0 {
		return 0
	}
	if value > 1 {
		return 1
	}
	return value
}
