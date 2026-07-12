package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pax-beehive/paxm/internal/eval"
	"github.com/pax-beehive/paxm/internal/memory"
)

func main() {
	challenge := eval.Suite{Version: eval.SuiteVersion, Name: "sqlite-retrieval-challenge-32"}
	challenge.Cases = append(challenge.Cases, morphologyCases()...)
	challenge.Cases = append(challenge.Cases, aliasCases()...)
	challenge.Cases = append(challenge.Cases, cjkCases()...)
	challenge.Cases = append(challenge.Cases, identifierCases()...)
	challenge.Cases = append(challenge.Cases, strictSuppressionCases()...)
	challenge.Cases = append(challenge.Cases, relaxedFallbackCases()...)
	challenge.Cases = append(challenge.Cases, noiseCases()...)

	workspace := eval.Suite{Version: eval.SuiteVersion, Name: "sqlite-retrieval-workspace-hard-gate-5", Cases: workspaceCases()}
	writeSuite("suite.json", challenge)
	writeSuite("workspace-suite.json", workspace)
}

func writeSuite(name string, suite eval.Suite) {
	data, err := json.MarshalIndent(suite, "", "  ")
	if err != nil {
		panic(err)
	}
	root, err := filepath.Abs(filepath.Join("evals", "sqlite-retrieval"))
	if err != nil {
		panic(err)
	}
	if err := os.WriteFile(filepath.Join(root, name), append(data, '\n'), 0o644); err != nil {
		panic(err)
	}
}

func workspaceCases() []eval.Case {
	projects := []struct{ name, region string }{{"atlas", "us-west-1"}, {"borealis", "eu-central-1"}, {"cinder", "ap-southeast-2"}, {"delta", "us-east-2"}, {"ember", "eu-west-1"}}
	result := make([]eval.Case, 0, len(projects))
	for i, project := range projects {
		id := fmt.Sprintf("workspace_scope-%02d", i+1)
		workspace := "/eval/workspace/" + project.name
		result = append(result, eval.Case{
			ID: id, Category: "workspace_scope",
			Memories: []eval.Memory{
				{ID: id + "-target", Text: fmt.Sprintf("Project %s deployment region is %s.", project.name, project.region), Tier: memory.TierLTM, Metadata: map[string]string{"workspace": workspace}},
				{ID: id + "-forbidden", Text: fmt.Sprintf("Project %s deployment region is us-east-1.", project.name), Tier: memory.TierLTM, Metadata: map[string]string{"workspace": "/eval/workspace/other-" + project.name}},
			},
			Recall:   eval.Recall{Mode: "active", Query: project.name + " deployment region", Limit: 3, Metadata: map[string]string{"workspace": workspace}},
			Expected: []string{id + "-target"}, Forbidden: []string{id + "-forbidden"},
		})
	}
	return result
}

func morphologyCases() []eval.Case {
	values := []struct{ stored, query string }{{"deployment application", "deploy application"}, {"configuration provider", "configure provider"}, {"authentication gateway", "authenticate gateway"}, {"retries delivery", "retry delivery"}}
	return pairedCases("morphology", values)
}

func aliasCases() []eval.Case {
	// Deliberately bounded product vocabulary, not general semantic expansion.
	values := []struct{ stored, query string }{{"repository migration", "repo migration"}, {"configuration file", "config file"}}
	return pairedCases("bounded_alias", values)
}

func pairedCases(category string, values []struct{ stored, query string }) []eval.Case {
	result := make([]eval.Case, 0, len(values))
	for i, value := range values {
		id := fmt.Sprintf("%s-%02d", category, i+1)
		result = append(result, eval.Case{
			ID: id, Category: category,
			Memories: []eval.Memory{
				{ID: id + "-target", Text: "Confirmed project decision about " + value.stored + ".", Tier: memory.TierLTM},
				{ID: id + "-forbidden", Text: "Unrelated temporary note about " + lastWord(value.query) + ".", Tier: memory.TierLTM},
			},
			Recall: eval.Recall{Mode: "active", Query: value.query, Limit: 1}, Expected: []string{id + "-target"}, ExpectedFirst: id + "-target", Forbidden: []string{id + "-forbidden"},
		})
	}
	return result
}

func cjkCases() []eval.Case {
	values := []struct{ stored, query, distractor string }{
		{"生产环境部署区域是美国西部一区", "部署区域", "旧讨论只提到部署，没有区域结论"},
		{"数据库迁移必须先执行结构变更", "数据库迁移", "数据库备份说明与迁移无关"},
		{"默认记忆提供方使用本地存储", "记忆提供方", "记忆清理任务没有提供方信息"},
		{"发布流程需要先验证校验和", "发布流程", "历史发布事故未记录流程"},
		{"被动召回不能阻塞智能体", "被动召回", "主动召回性能记录不是被动路径"},
		{"工作区过滤必须阻止跨项目结果", "工作区过滤", "工作区索引说明没有过滤规则"},
	}
	result := make([]eval.Case, 0, len(values))
	for i, value := range values {
		id := fmt.Sprintf("cjk_substring-%02d", i+1)
		result = append(result, eval.Case{
			ID: id, Category: "cjk_substring",
			Memories: []eval.Memory{{ID: id + "-target", Text: value.stored, Tier: memory.TierLTM}, {ID: id + "-forbidden", Text: value.distractor, Tier: memory.TierLTM}},
			Recall:   eval.Recall{Mode: "active", Query: value.query, Limit: 1}, Expected: []string{id + "-target"}, ExpectedFirst: id + "-target", Forbidden: []string{id + "-forbidden"},
		})
	}
	return result
}

func identifierCases() []eval.Case {
	values := []struct{ stored, query, distractor string }{
		{"providerSearchTimeout", "provider search timeout", "providerSearchRetries"},
		{"capture_queue_retry_min", "capture queue retry min", "capture_queue_retry_max"},
		{"PAXM_INSTALL_DIR", "paxm install dir", "PAXM_CONFIG_DIR"},
		{"remember batch to provider", "rememberBatchToProvider", "rememberBatchToArchive"},
		{"passive write latency total", "passiveWriteLatencyTotal", "active write latency total"},
		{"internal/provider/sqlite/search.go", "provider sqlite search", "internal/provider/mem0/search.go"},
		{"release v0.9.14 checksum", "v0.9.14 checksum", "release v0.9.13 checksum"},
		{"error code PAXM_E_RECALL_TIMEOUT", "paxm recall timeout", "error code PAXM_E_WRITE_TIMEOUT"},
		{"workspaceFilterEnabled", "workspace_filter_enabled", "workspaceFilterOptional"},
	}
	result := make([]eval.Case, 0, len(values))
	for i, value := range values {
		id := fmt.Sprintf("identifier_split-%02d", i+1)
		result = append(result, eval.Case{
			ID: id, Category: "identifier_split",
			Memories: []eval.Memory{{ID: id + "-target", Text: value.stored + " is the relevant identifier.", Tier: memory.TierLTM}, {ID: id + "-forbidden", Text: value.distractor + " is unrelated.", Tier: memory.TierLTM}},
			Recall:   eval.Recall{Mode: "active", Query: value.query, Limit: 1}, Expected: []string{id + "-target"}, ExpectedFirst: id + "-target", Forbidden: []string{id + "-forbidden"},
		})
	}
	return result
}

func strictSuppressionCases() []eval.Case {
	values := []string{"provider timeout bulkhead", "release checksum verification", "sqlite recall ranking"}
	result := make([]eval.Case, 0, len(values))
	for i, query := range values {
		id := fmt.Sprintf("strict_suppresses_relaxed-%02d", i+1)
		result = append(result, eval.Case{
			ID: id, Category: "strict_suppresses_relaxed",
			Memories: []eval.Memory{{ID: id + "-target", Text: "Confirmed decision: " + query + ".", Tier: memory.TierLTM}, {ID: id + "-forbidden-a", Text: "Unrelated note mentioning " + firstWord(query) + ".", Tier: memory.TierLTM}, {ID: id + "-forbidden-b", Text: "Unrelated note mentioning " + lastWord(query) + ".", Tier: memory.TierLTM}},
			Recall:   eval.Recall{Mode: "active", Query: query, Limit: 3}, Expected: []string{id + "-target"}, Forbidden: []string{id + "-forbidden-a", id + "-forbidden-b"},
		})
	}
	return result
}

func relaxedFallbackCases() []eval.Case {
	values := []string{"provider timeout bulkhead", "release checksum verification", "capture episode ordering"}
	result := make([]eval.Case, 0, len(values))
	for i, query := range values {
		parts := words(query)
		id := fmt.Sprintf("relaxed_fallback-%02d", i+1)
		result = append(result, eval.Case{
			ID: id, Category: "relaxed_fallback",
			Memories: []eval.Memory{{ID: id + "-target", Text: "Confirmed decision about " + parts[0] + " " + parts[1] + ".", Tier: memory.TierLTM}, {ID: id + "-forbidden", Text: "Unrelated note mentioning only " + parts[0] + ".", Tier: memory.TierLTM}},
			Recall:   eval.Recall{Mode: "active", Query: query, Limit: 1}, Expected: []string{id + "-target"}, ExpectedFirst: id + "-target", Forbidden: []string{id + "-forbidden"},
		})
	}
	return result
}

func noiseCases() []eval.Case {
	values := []string{"deployment region us-west-1", "provider timeout 250ms", "sqlite database location", "release checksum required", "capture retry ordering"}
	result := make([]eval.Case, 0, len(values))
	for i, query := range values {
		id := fmt.Sprintf("long_noise-%02d", i+1)
		result = append(result, eval.Case{
			ID: id, Category: "long_noise",
			Memories: []eval.Memory{{ID: id + "-target", Text: "Decision: " + query + ".", Tier: memory.TierLTM}, {ID: id + "-forbidden", Text: "Historical transcript with " + firstWord(query) + " discussion, unrelated tool output, " + middleWord(query) + " experiments, and an obsolete " + lastWord(query) + " note.", Tier: memory.TierLTM}},
			Recall:   eval.Recall{Mode: "active", Query: query, Limit: 3}, Expected: []string{id + "-target"}, Forbidden: []string{id + "-forbidden"},
		})
	}
	return result
}

func words(value string) []string    { return strings.Fields(value) }
func firstWord(value string) string  { return words(value)[0] }
func lastWord(value string) string   { values := words(value); return values[len(values)-1] }
func middleWord(value string) string { values := words(value); return values[len(values)/2] }
