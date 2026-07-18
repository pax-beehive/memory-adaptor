package cli

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/pax-beehive/paxm/internal/adapters"
	jsonrpcadapter "github.com/pax-beehive/paxm/internal/adapters/jsonrpc"
	jsonrpcconformance "github.com/pax-beehive/paxm/internal/adapters/jsonrpc/conformance"
	"github.com/pax-beehive/paxm/internal/config"
	paxeval "github.com/pax-beehive/paxm/internal/eval"
	paxruntime "github.com/pax-beehive/paxm/internal/runtime"
)

func (r runner) runEval(args []string) error {
	if len(args) > 0 && args[0] == "cleanup" {
		return r.runEvalCleanup(args[1:])
	}
	if len(args) > 1 && args[0] == "retrieval" && args[1] == "locomo" {
		return r.runLoCoMoEval(args[2:])
	}
	if len(args) > 1 && args[0] == "provider" && args[1] == "jsonrpc" {
		return r.runJSONRPCConformance(args[2:])
	}
	if len(args) == 0 || args[0] != "run" {
		return errors.New("usage: paxm eval run locomo --agent NAME [options] | paxm eval retrieval locomo [options] | paxm eval provider jsonrpc --command PATH")
	}
	if len(args) > 1 && args[1] == "locomo" {
		return r.runLoCoMoAgentEval(args[2:])
	}
	fs := flag.NewFlagSet("eval run", flag.ContinueOnError)
	fs.SetOutput(r.stderr)
	suitePath := fs.String("suite", "evals/baseline", "suite file or directory")
	jsonOut := fs.Bool("json", false, "write JSON")
	comparePath := fs.String("compare", "", "compare with a prior result JSON")
	budgetPath := fs.String("budget", "", "enforce a regression budget JSON")
	outputPath := fs.String("output", "", "write the current result JSON")
	gate := fs.String("gate", "none", "failure policy: none, adapter, or quality")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if *gate != "quality" && *gate != "adapter" && *gate != "none" {
		return fmt.Errorf("unsupported eval gate %q", *gate)
	}
	if *gate == "adapter" && *budgetPath != "" {
		return errors.New("--budget measures provider quality and cannot be used with --gate adapter")
	}
	if *gate == "none" && *budgetPath != "" {
		return errors.New("--budget cannot be enforced with --gate none")
	}
	suite, err := paxeval.Load(*suitePath)
	if err != nil {
		return err
	}
	result, err := (paxeval.Runner{}).Run(context.Background(), suite)
	if err != nil {
		return err
	}
	var comparison *paxeval.Comparison
	if *comparePath != "" {
		baseline, loadErr := paxeval.LoadResult(*comparePath)
		if loadErr != nil {
			return loadErr
		}
		value, compareErr := paxeval.Compare(baseline, result)
		if compareErr != nil {
			return compareErr
		}
		comparison = &value
	}
	var budgetFailures []string
	if *budgetPath != "" {
		budget, loadErr := paxeval.LoadBudget(*budgetPath)
		if loadErr != nil {
			return loadErr
		}
		budgetFailures = paxeval.CheckBudget(result, budget)
	}
	if *outputPath != "" {
		data, marshalErr := json.MarshalIndent(result, "", "  ")
		if marshalErr != nil {
			return marshalErr
		}
		if writeErr := os.WriteFile(*outputPath, append(data, '\n'), 0o600); writeErr != nil {
			return writeErr
		}
	}
	if *jsonOut && (comparison != nil || *budgetPath != "") {
		err = writeJSON(r.stdout, struct {
			Result         paxeval.Result      `json:"result"`
			Comparison     *paxeval.Comparison `json:"comparison,omitempty"`
			BudgetFailures []string            `json:"budget_failures,omitempty"`
		}{result, comparison, budgetFailures})
	} else if *jsonOut {
		err = writeJSON(r.stdout, result)
	} else {
		writeEvalReport(r.stdout, result)
		if comparison != nil {
			writeEvalComparison(r.stdout, *comparison)
		}
		for _, failure := range budgetFailures {
			_, _ = fmt.Fprintf(r.stdout, "BUDGET FAIL: %s\n", failure)
		}
	}
	if err != nil {
		return err
	}
	if *gate == "adapter" {
		if result.AdapterContractCases == 0 {
			return errors.New("adapter gate requires a suite with conversation writes")
		}
		if result.ExecutionFailed > 0 {
			return fmt.Errorf("eval execution failed: %d cases had runtime or provider errors", result.ExecutionFailed)
		}
		if result.AdapterContractFailed > 0 {
			return fmt.Errorf("adapter contract failed: %d of %d cases failed", result.AdapterContractFailed, result.AdapterContractCases)
		}
		return nil
	}
	if *gate == "none" {
		if result.ExecutionFailed > 0 {
			return fmt.Errorf("eval execution failed: %d cases had runtime or provider errors", result.ExecutionFailed)
		}
		return nil
	}
	if result.Failed > 0 {
		return fmt.Errorf("eval failed: %d of %d cases failed", result.Failed, result.CaseCount)
	}
	if len(budgetFailures) > 0 {
		return fmt.Errorf("eval regression budget failed: %d metrics outside budget", len(budgetFailures))
	}
	return nil
}

type stringListFlag []string

func (v *stringListFlag) String() string         { return strings.Join(*v, ",") }
func (v *stringListFlag) Set(value string) error { *v = append(*v, value); return nil }

func (r runner) runJSONRPCConformance(args []string) error {
	fs := flag.NewFlagSet("eval provider jsonrpc", flag.ContinueOnError)
	fs.SetOutput(r.stderr)
	command := fs.String("command", "", "provider executable")
	timeout := fs.Duration("timeout", 10*time.Second, "timeout per RPC call")
	jsonOut := fs.Bool("json", false, "write JSON")
	var commandArgs stringListFlag
	fs.Var(&commandArgs, "arg", "provider argument; repeat as needed")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*command) == "" {
		return errors.New("JSON-RPC conformance requires --command PATH")
	}
	provider, err := jsonrpcadapter.New("conformance", config.ProviderConfig{Transport: "stdio", Command: *command, Args: commandArgs, Timeout: timeout.String()})
	if err != nil {
		return err
	}
	result := jsonrpcconformance.Run(context.Background(), provider)
	if *jsonOut {
		if err := writeJSON(r.stdout, result); err != nil {
			return err
		}
	} else {
		_, _ = fmt.Fprintf(r.stdout, "paxm JSON-RPC provider conformance: passed=%t protocol=%s\n", result.Passed, result.Protocol)
		for _, check := range result.Checks {
			status := "PASS"
			if check.Skipped {
				status = "SKIP"
			} else if !check.Passed {
				status = "FAIL"
			}
			_, _ = fmt.Fprintf(r.stdout, "  %-5s %s", status, check.Name)
			if check.Error != "" {
				_, _ = fmt.Fprintf(r.stdout, ": %s", check.Error)
			}
			_, _ = fmt.Fprintln(r.stdout)
		}
	}
	if !result.Passed {
		return errors.New("JSON-RPC provider failed required conformance checks")
	}
	return nil
}

func (r runner) runEvalCleanup(args []string) error {
	fs := flag.NewFlagSet("eval cleanup", flag.ContinueOnError)
	fs.SetOutput(r.stderr)
	runID := fs.String("run", "", "run id or run id prefix to clean")
	stale := fs.Bool("stale", false, "clean all non-cleaned manifests not marked keep-memory")
	manifestDir := fs.String("manifest-dir", filepath.Join(filepath.Dir(config.DefaultDataPath()), "eval-runs"), "eval run manifest directory")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*runID) == "" && !*stale {
		return errors.New("eval cleanup requires --run ID or --stale")
	}
	entries, err := os.ReadDir(*manifestDir)
	if err != nil {
		return err
	}
	cfg, err := config.Load(paxruntime.ConfigFile(r.configPath))
	if err != nil {
		return err
	}
	registry := adapters.DefaultRegistry()
	cleaned := 0
	var cleanupErr error
	for _, entry := range entries {
		if !entry.IsDir() || (*runID != "" && entry.Name() != *runID && !strings.HasPrefix(entry.Name(), *runID+"-")) {
			continue
		}
		manifestPath := filepath.Join(*manifestDir, entry.Name(), "manifest.json")
		scope, restoreErr := paxeval.RestoreProviderScope(cfg, manifestPath)
		if restoreErr != nil {
			cleanupErr = errors.Join(cleanupErr, restoreErr)
			continue
		}
		if *stale && *runID == "" && scope.Manifest.Status == paxeval.EvalStatusCleaned {
			continue
		}
		if *stale && *runID == "" && scope.Manifest.KeepMemory {
			continue
		}
		if *runID != "" {
			scope.Manifest.KeepMemory = false
		}
		provider, buildErr := registry.BuildProvider(scope.Manifest.Provider, scope.Config.Providers[scope.Manifest.Provider])
		if buildErr != nil {
			cleanupErr = errors.Join(cleanupErr, buildErr)
			continue
		}
		if err := paxeval.CleanupProviderScope(context.Background(), scope, provider); err != nil {
			cleanupErr = errors.Join(cleanupErr, fmt.Errorf("cleanup %s: %w", entry.Name(), err))
			continue
		}
		cleaned++
		_, _ = fmt.Fprintf(r.stdout, "cleaned eval run: %s\n", entry.Name())
	}
	if cleanupErr != nil {
		return cleanupErr
	}
	if cleaned == 0 {
		return errors.New("no matching eval runs required cleanup")
	}
	return nil
}

func (r runner) runLoCoMoAgentEval(args []string) error {
	fs := flag.NewFlagSet("eval run locomo", flag.ContinueOnError)
	fs.SetOutput(r.stderr)
	datasetPath := fs.String("dataset", "", "path to the official locomo10.json dataset")
	agentName := fs.String("agent", "", "agent runtime to evaluate (opencode)")
	agentBinary := fs.String("agent-binary", "", "agent executable path")
	model := fs.String("model", "", "agent model override")
	providerName := fs.String("provider", "sqlite", "configured provider name")
	armsValue := fs.String("arms", "control,passive,active", "comma-separated control, passive, active arms")
	maxQuestions := fs.Int("max-questions", 0, "limit paid agent questions")
	allQuestions := fs.Bool("all", false, "run every eligible LoCoMo question")
	matchThreshold := fs.Float64("match-threshold", 0.5, "token-F1 threshold for a matched answer")
	manifestDir := fs.String("manifest-dir", filepath.Join(filepath.Dir(config.DefaultDataPath()), "eval-runs"), "eval run manifest directory")
	runID := fs.String("run-id", "", "stable eval run id")
	settle := fs.Duration("settle", 0, "wait after ingest before agent questions")
	timeout := fs.Duration("timeout", 3*time.Minute, "timeout for each agent call")
	keepMemory := fs.Bool("keep-memory", false, "intentionally retain benchmark memories")
	jsonOut := fs.Bool("json", false, "write JSON")
	outputPath := fs.String("output", "", "write result JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*datasetPath) == "" || strings.TrimSpace(*agentName) == "" {
		return errors.New("LoCoMo agent evaluation requires --dataset PATH and --agent NAME")
	}
	if strings.TrimSpace(*model) == "" {
		return errors.New("LoCoMo agent evaluation requires --model PROVIDER/MODEL so runs are reproducible")
	}
	if *maxQuestions <= 0 && !*allQuestions {
		return errors.New("LoCoMo agent evaluation makes paid model calls; choose --max-questions N or explicitly pass --all")
	}
	if *maxQuestions < 0 || *matchThreshold <= 0 || *matchThreshold > 1 || *timeout <= 0 {
		return errors.New("invalid LoCoMo agent evaluation limits")
	}
	arms, err := parseAgentArms(*armsValue)
	if err != nil {
		return err
	}
	cfg, err := config.Load(paxruntime.ConfigFile(r.configPath))
	if err != nil {
		return err
	}
	dataset, err := paxeval.LoadLoCoMo(*datasetPath)
	if err != nil {
		return err
	}
	if strings.TrimSpace(*runID) == "" {
		*runID = newHookEventID()
	}
	executor := r.agentExecutor
	if executor == nil {
		if *agentName != "opencode" {
			return fmt.Errorf("agent %q is not supported", *agentName)
		}
		binary, findErr := findOpenCodeBinary(*agentBinary)
		if findErr != nil {
			return findErr
		}
		paxmBinary, executableErr := os.Executable()
		if executableErr != nil {
			return executableErr
		}
		executor = paxeval.OpenCodeExecutor{Binary: binary, PaxmBinary: paxmBinary, Model: *model, Timeout: *timeout}
	}
	registry := adapters.DefaultRegistry()
	result, runErr := (paxeval.LoCoMoAgentRunner{BuildProvider: registry.BuildProvider, Agent: executor}).Run(context.Background(), dataset, paxeval.LoCoMoAgentOptions{
		Config: cfg, Provider: *providerName, RunID: *runID, ManifestDir: *manifestDir,
		AgentName: *agentName, Arms: arms, MaxQuestions: *maxQuestions, MatchThreshold: *matchThreshold,
		KeepMemory: *keepMemory, Settle: *settle,
	})
	if *outputPath != "" {
		data, marshalErr := json.MarshalIndent(result, "", "  ")
		if marshalErr != nil {
			return errors.Join(runErr, marshalErr)
		}
		if writeErr := os.WriteFile(*outputPath, append(data, '\n'), 0o600); writeErr != nil {
			return errors.Join(runErr, writeErr)
		}
	}
	if *jsonOut {
		if err := writeJSON(r.stdout, result); err != nil {
			return errors.Join(runErr, err)
		}
	} else {
		writeLoCoMoAgentReport(r.stdout, result)
	}
	if runErr != nil {
		return runErr
	}
	for _, summary := range result.Summaries {
		if summary.Errors > 0 {
			return fmt.Errorf("LoCoMo agent eval %s arm had %d execution errors", summary.Arm, summary.Errors)
		}
	}
	return nil
}

func parseAgentArms(value string) ([]paxeval.AgentArm, error) {
	seen := make(map[paxeval.AgentArm]bool)
	var arms []paxeval.AgentArm
	for _, item := range strings.Split(value, ",") {
		arm := paxeval.AgentArm(strings.TrimSpace(item))
		switch arm {
		case paxeval.AgentArmControl, paxeval.AgentArmPassive, paxeval.AgentArmActive:
		default:
			return nil, fmt.Errorf("unsupported eval arm %q", item)
		}
		if !seen[arm] {
			seen[arm] = true
			arms = append(arms, arm)
		}
	}
	if len(arms) == 0 {
		return nil, errors.New("at least one eval arm is required")
	}
	return arms, nil
}

func findOpenCodeBinary(explicit string) (string, error) {
	if strings.TrimSpace(explicit) != "" {
		return config.ExpandPath(explicit), nil
	}
	if path, err := exec.LookPath("opencode"); err == nil {
		return path, nil
	}
	home, _ := os.UserHomeDir()
	candidate := filepath.Join(home, ".opencode", "bin", "opencode")
	if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
		return candidate, nil
	}
	return "", errors.New("OpenCode binary not found; pass --agent-binary PATH")
}

func writeLoCoMoAgentReport(w io.Writer, result paxeval.LoCoMoAgentResult) {
	_, _ = fmt.Fprintf(w, "paxm eval: %s  agent=%s  provider=%s  model=%s\n", result.Benchmark, result.Agent, result.Provider, result.Model)
	_, _ = fmt.Fprintf(w, "  agent write canary: %t\n", result.WriteCanary)
	_, _ = fmt.Fprintf(w, "  questions: %d  trials: %d  duration: %s\n", result.QuestionCount, result.TrialCount, time.Duration(result.DurationMS)*time.Millisecond)
	for _, summary := range result.Summaries {
		_, _ = fmt.Fprintf(w, "  %-7s accuracy %.1f%%  mean-f1 %.3f  exact %.1f%%  recall-used %d/%d  useful %.1f%%  errors %d  tokens %d/%d  cost $%.4f\n",
			summary.Arm, summary.Accuracy*100, summary.MeanF1, summary.ExactMatch*100, summary.RecallUsed, summary.Trials, summary.UsefulRecallRate*100,
			summary.Errors, summary.InputTokens, summary.OutputTokens, summary.Cost)
	}
	_, _ = fmt.Fprintf(w, "  memory lift: passive %+.1fpp  active %+.1fpp\n", result.PassiveLift*100, result.ActiveLift*100)
}

func (r runner) runLoCoMoEval(args []string) error {
	fs := flag.NewFlagSet("eval retrieval locomo", flag.ContinueOnError)
	fs.SetOutput(r.stderr)
	datasetPath := fs.String("dataset", "", "path to the official locomo10.json dataset")
	providerName := fs.String("provider", "sqlite", "configured provider name")
	manifestDir := fs.String("manifest-dir", filepath.Join(filepath.Dir(config.DefaultDataPath()), "eval-runs"), "eval run manifest directory")
	runID := fs.String("run-id", "", "stable eval run id")
	limit := fs.Int("limit", 10, "retrieval result limit")
	settle := fs.Duration("settle", 0, "wait after ingest before recall")
	keepMemory := fs.Bool("keep-memory", false, "intentionally retain benchmark memories")
	jsonOut := fs.Bool("json", false, "write JSON")
	outputPath := fs.String("output", "", "write result JSON")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if strings.TrimSpace(*datasetPath) == "" {
		return errors.New("LoCoMo evaluation requires --dataset PATH")
	}
	if *limit <= 0 {
		return errors.New("LoCoMo --limit must be positive")
	}
	cfg, err := config.Load(paxruntime.ConfigFile(r.configPath))
	if err != nil {
		return err
	}
	dataset, err := paxeval.LoadLoCoMo(*datasetPath)
	if err != nil {
		return err
	}
	if strings.TrimSpace(*runID) == "" {
		*runID = newHookEventID()
	}
	registry := adapters.DefaultRegistry()
	result, runErr := (paxeval.LoCoMoRunner{BuildProvider: registry.BuildProvider}).Run(context.Background(), dataset, paxeval.LoCoMoRunOptions{
		Config: cfg, Provider: *providerName, RunID: *runID, ManifestDir: *manifestDir,
		Limit: *limit, KeepMemory: *keepMemory, Settle: *settle,
	})
	if *outputPath != "" {
		data, marshalErr := json.MarshalIndent(result, "", "  ")
		if marshalErr != nil {
			return errors.Join(runErr, marshalErr)
		}
		if writeErr := os.WriteFile(*outputPath, append(data, '\n'), 0o600); writeErr != nil {
			return errors.Join(runErr, writeErr)
		}
	}
	if *jsonOut {
		if err := writeJSON(r.stdout, result); err != nil {
			return errors.Join(runErr, err)
		}
	} else {
		writeLoCoMoReport(r.stdout, result)
	}
	if runErr != nil {
		return runErr
	}
	if result.ExecutionFailed > 0 {
		return fmt.Errorf("LoCoMo eval execution failed for %d questions", result.ExecutionFailed)
	}
	return nil
}

func writeLoCoMoReport(w io.Writer, result paxeval.LoCoMoResult) {
	_, _ = fmt.Fprintf(w, "paxm eval: %s (%s)\n", result.Benchmark, result.Provider)
	_, _ = fmt.Fprintf(w, "  conversations: %d  questions: %d  passed: %d  failed: %d\n", result.ConversationCount, result.QuestionCount, result.Passed, result.Failed)
	_, _ = fmt.Fprintf(w, "  recall@k: %.3f  precision@k: %.3f  mrr: %.3f  duration: %dms\n", result.RecallAtK, result.PrecisionAtK, result.MRR, result.DurationMS)
	for _, category := range result.Categories {
		_, _ = fmt.Fprintf(w, "  category %d: %d/%d  recall@k %.3f  precision@k %.3f  mrr %.3f\n", category.Category, category.Passed, category.Questions, category.RecallAtK, category.PrecisionAtK, category.MRR)
	}
}

func writeEvalComparison(w io.Writer, comparison paxeval.Comparison) {
	_, _ = fmt.Fprintf(w, "comparison: %s -> %s\n", comparison.BaselineSuite, comparison.CurrentSuite)
	_, _ = fmt.Fprintf(w, "  passed %+d  recall@k %+.3f  precision@k %+.3f  mrr %+.3f  false-positive rate %+.3f  duration %+dms\n", comparison.PassedDelta, comparison.RecallAtKDelta, comparison.PrecisionAtKDelta, comparison.MRRDelta, comparison.FalsePositiveRateDelta, comparison.DurationMSDelta)
	if comparison.WriteRecallDelta != 0 || comparison.WritePrecisionDelta != 0 || comparison.WriteFalsePositiveRateDelta != 0 {
		_, _ = fmt.Fprintf(w, "  write recall %+.3f  write precision %+.3f  write false-positive rate %+.3f\n", comparison.WriteRecallDelta, comparison.WritePrecisionDelta, comparison.WriteFalsePositiveRateDelta)
	}
}

func writeEvalReport(w io.Writer, result paxeval.Result) {
	_, _ = fmt.Fprintf(w, "paxm eval: %s (v%d)\n", result.Suite, result.Version)
	_, _ = fmt.Fprintf(w, "cases: %d  passed: %d  failed: %d  duration: %dms\n", result.CaseCount, result.Passed, result.Failed, result.DurationMS)
	if result.ExecutionFailed > 0 {
		_, _ = fmt.Fprintf(w, "execution failures: %d\n", result.ExecutionFailed)
	}
	_, _ = fmt.Fprintf(w, "recall@k: %.3f  precision@k: %.3f  mrr: %.3f  false-positive rate: %.3f\n", result.RecallAtK, result.PrecisionAtK, result.MRR, result.FalsePositiveRate)
	if result.AdapterContractCases > 0 {
		_, _ = fmt.Fprintf(w, "adapter contract: %d/%d passed  failed: %d\n", result.AdapterContractPassed, result.AdapterContractCases, result.AdapterContractFailed)
	}
	if result.WriteCaseCount > 0 {
		_, _ = fmt.Fprintf(w, "writes: %d/%d  write recall: %.3f  write precision: %.3f  write false-positive rate: %.3f\n", result.Writes, result.WriteCaseCount, result.WriteRecall, result.WritePrecision, result.WriteFalsePositiveRate)
		_, _ = fmt.Fprintf(w, "results: %d  returned context: %d bytes  write total: %.3fms  recall total: %.3fms\n", result.ResultCount, result.ReturnedContextBytes, float64(result.WriteDurationUS)/1000, float64(result.RecallDurationUS)/1000)
	}
	for _, group := range result.Categories {
		_, _ = fmt.Fprintf(w, "  %-20s %3d/%-3d  recall@k %.3f  precision@k %.3f  mrr %.3f\n", group.Name, group.Passed, group.CaseCount, group.RecallAtK, group.PrecisionAtK, group.MRR)
		if group.WriteCaseCount > 0 {
			_, _ = fmt.Fprintf(w, "  %-20s write recall %.3f  write precision %.3f  write false-positive rate %.3f\n", "", group.WriteRecall, group.WritePrecision, group.WriteFalsePositiveRate)
		}
	}
	for _, item := range result.Cases {
		if item.Passed {
			continue
		}
		_, _ = fmt.Fprintf(w, "FAIL %s", item.ID)
		if item.Error != "" {
			_, _ = fmt.Fprintf(w, ": %s", item.Error)
		}
		if len(item.Missing) > 0 {
			_, _ = fmt.Fprintf(w, " missing=%s", strings.Join(item.Missing, ","))
		}
		if len(item.Forbidden) > 0 {
			_, _ = fmt.Fprintf(w, " forbidden=%s", strings.Join(item.Forbidden, ","))
		}
		if len(item.Unexpected) > 0 {
			_, _ = fmt.Fprintf(w, " unexpected=%s", strings.Join(item.Unexpected, ","))
		}
		if len(item.WriteMissing) > 0 {
			_, _ = fmt.Fprintf(w, " write-missing=%s", strings.Join(item.WriteMissing, ","))
		}
		if len(item.WriteForbidden) > 0 {
			_, _ = fmt.Fprintf(w, " write-forbidden=%s", strings.Join(item.WriteForbidden, ","))
		}
		if len(item.MetadataMismatches) > 0 {
			_, _ = fmt.Fprintf(w, " metadata=%s", strings.Join(item.MetadataMismatches, ","))
		}
		if len(item.AdapterContractErrors) > 0 {
			_, _ = fmt.Fprintf(w, " adapter=%s", strings.Join(item.AdapterContractErrors, ","))
		}
		_, _ = fmt.Fprintln(w)
	}
}
