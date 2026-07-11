package eval

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
)

type Comparison struct {
	BaselineSuite               string  `json:"baseline_suite"`
	CurrentSuite                string  `json:"current_suite"`
	PassedDelta                 int     `json:"passed_delta"`
	RecallAtKDelta              float64 `json:"recall_at_k_delta"`
	PrecisionAtKDelta           float64 `json:"precision_at_k_delta"`
	MRRDelta                    float64 `json:"mrr_delta"`
	FalsePositiveRateDelta      float64 `json:"false_positive_rate_delta"`
	WriteRecallDelta            float64 `json:"write_recall_delta,omitempty"`
	WritePrecisionDelta         float64 `json:"write_precision_delta,omitempty"`
	WriteFalsePositiveRateDelta float64 `json:"write_false_positive_rate_delta,omitempty"`
	DurationMSDelta             int64   `json:"duration_ms_delta"`
}

type Budget struct {
	MinPassRate               float64 `json:"min_pass_rate"`
	MinRecallAtK              float64 `json:"min_recall_at_k"`
	MinPrecisionAtK           float64 `json:"min_precision_at_k"`
	MinMRR                    float64 `json:"min_mrr"`
	MaxFalsePositiveRate      float64 `json:"max_false_positive_rate"`
	MinWriteRecall            float64 `json:"min_write_recall,omitempty"`
	MinWritePrecision         float64 `json:"min_write_precision,omitempty"`
	MaxWriteFalsePositiveRate float64 `json:"max_write_false_positive_rate,omitempty"`
}

func Compare(baseline, current Result) (Comparison, error) {
	if baseline.Suite != current.Suite || baseline.Version != current.Version || baseline.CaseCount != current.CaseCount {
		return Comparison{}, fmt.Errorf("cannot compare incompatible eval results: %s v%d (%d cases) vs %s v%d (%d cases)", baseline.Suite, baseline.Version, baseline.CaseCount, current.Suite, current.Version, current.CaseCount)
	}
	return Comparison{
		BaselineSuite: baseline.Suite, CurrentSuite: current.Suite,
		PassedDelta:                 current.Passed - baseline.Passed,
		RecallAtKDelta:              current.RecallAtK - baseline.RecallAtK,
		PrecisionAtKDelta:           current.PrecisionAtK - baseline.PrecisionAtK,
		MRRDelta:                    current.MRR - baseline.MRR,
		FalsePositiveRateDelta:      current.FalsePositiveRate - baseline.FalsePositiveRate,
		WriteRecallDelta:            current.WriteRecall - baseline.WriteRecall,
		WritePrecisionDelta:         current.WritePrecision - baseline.WritePrecision,
		WriteFalsePositiveRateDelta: current.WriteFalsePositiveRate - baseline.WriteFalsePositiveRate,
		DurationMSDelta:             current.DurationMS - baseline.DurationMS,
	}, nil
}

func LoadResult(path string) (Result, error) {
	var result Result
	return result, loadStrictJSON(path, &result)
}

func LoadBudget(path string) (Budget, error) {
	var budget Budget
	if err := loadStrictJSON(path, &budget); err != nil {
		return Budget{}, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return Budget{}, err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return Budget{}, err
	}
	for _, required := range []string{"min_pass_rate", "min_recall_at_k", "min_precision_at_k", "min_mrr", "max_false_positive_rate"} {
		if _, ok := fields[required]; !ok {
			return Budget{}, fmt.Errorf("budget %s is required", required)
		}
	}
	for name, value := range map[string]float64{
		"min_pass_rate": budget.MinPassRate, "min_recall_at_k": budget.MinRecallAtK,
		"min_precision_at_k": budget.MinPrecisionAtK, "min_mrr": budget.MinMRR,
		"max_false_positive_rate": budget.MaxFalsePositiveRate, "min_write_recall": budget.MinWriteRecall,
		"min_write_precision": budget.MinWritePrecision, "max_write_false_positive_rate": budget.MaxWriteFalsePositiveRate,
	} {
		if value < 0 || value > 1 {
			return Budget{}, fmt.Errorf("budget %s must be between 0 and 1", name)
		}
	}
	return budget, nil
}

func CheckBudget(result Result, budget Budget) []string {
	var failures []string
	passRate := 0.0
	if result.CaseCount > 0 {
		passRate = float64(result.Passed) / float64(result.CaseCount)
	}
	checkMin := func(name string, actual, minimum float64) {
		if actual < minimum {
			failures = append(failures, fmt.Sprintf("%s %.3f is below %.3f", name, actual, minimum))
		}
	}
	checkMax := func(name string, actual, maximum float64) {
		if actual > maximum {
			failures = append(failures, fmt.Sprintf("%s %.3f exceeds %.3f", name, actual, maximum))
		}
	}
	checkMin("pass rate", passRate, budget.MinPassRate)
	checkMin("recall@k", result.RecallAtK, budget.MinRecallAtK)
	checkMin("precision@k", result.PrecisionAtK, budget.MinPrecisionAtK)
	checkMin("mrr", result.MRR, budget.MinMRR)
	checkMax("false-positive rate", result.FalsePositiveRate, budget.MaxFalsePositiveRate)
	if result.WriteCaseCount > 0 {
		checkMin("write recall", result.WriteRecall, budget.MinWriteRecall)
		checkMin("write precision", result.WritePrecision, budget.MinWritePrecision)
		checkMax("write false-positive rate", result.WriteFalsePositiveRate, budget.MaxWriteFalsePositiveRate)
	}
	return failures
}

func loadStrictJSON(path string, target any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("decode %s: %w", path, err)
	}
	return nil
}
