// Package retrieval owns SQLite lexical recall. Its narrow request/result API
// keeps query planning, SQL, scoring, and ranking out of the provider facade.
package retrieval

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
)

const RawScoreKind = "sqlite_fts_bm25_negated"

type Request struct {
	Text      string
	Limit     int
	Tiers     []string
	Workspace string
}

type Hit struct {
	ID           string
	Text         string
	Source       string
	Metadata     map[string]string
	CreatedAt    time.Time
	Tier         string
	ExpiresAt    *time.Time
	Score        float64
	RawScore     *float64
	RawScoreKind string
}

func Search(ctx context.Context, db *sql.DB, request Request) ([]Hit, error) {
	terms := analyze(request.Text)
	if len(terms) == 0 {
		return searchRecent(ctx, db, request)
	}
	return searchFTS(ctx, db, request, terms)
}

func searchRecent(ctx context.Context, db *sql.DB, request Request) ([]Hit, error) {
	filterSQL, filterArgs := filterClause(request, "m")
	args := append(filterArgs, resultLimit(request.Limit))
	rows, err := db.QueryContext(ctx, `
	SELECT m.id, m.text, m.source, m.metadata_json, m.created_at, m.tier, m.expires_at
	FROM memories AS m
	WHERE `+filterSQL+`
	ORDER BY m.created_at DESC
	LIMIT ?
	`, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var hits []Hit
	for rows.Next() {
		hit, err := scanHit(rows, nil)
		if err != nil {
			return nil, err
		}
		hit.Score = 0.1
		hits = append(hits, hit)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return hits, nil
}

func searchFTS(ctx context.Context, db *sql.DB, request Request, terms analysis) ([]Hit, error) {
	filterSQL, filterArgs := filterClause(request, "m")
	args := append([]any{matchQuery(terms.canonicalTerms())}, filterArgs...)
	args = append(args, candidateLimit(request.Limit))
	hits, err := queryCandidates(ctx, db, `
	SELECT m.id, m.text, m.source, m.metadata_json, m.created_at, m.tier, m.expires_at, bm25(memory_fts) AS rank
	FROM memory_fts
	JOIN memories AS m ON m.rowid = memory_fts.rowid
	WHERE memory_fts MATCH ? AND `+filterSQL+`
	ORDER BY rank ASC, m.created_at DESC
	LIMIT ?
	`, true, args...)
	if err != nil {
		return nil, err
	}
	queryTerms := terms.canonicalTerms()
	if terms.needsSupplement(request.Text) && !hasEnoughExactMatches(hits, queryTerms, resultLimit(request.Limit)) {
		likeSQL, likeArgs := likeClause(terms, "m")
		likeArgs = append(likeArgs, filterArgs...)
		likeArgs = append(likeArgs, candidateLimit(request.Limit))
		likeHits, err := queryCandidates(ctx, db, `
	SELECT m.id, m.text, m.source, m.metadata_json, m.created_at, m.tier, m.expires_at
	FROM memories AS m
	WHERE (`+likeSQL+`) AND `+filterSQL+`
	ORDER BY m.created_at DESC
	LIMIT ?
	`, false, likeArgs...)
		if err != nil {
			return nil, err
		}
		hits = mergeHits(hits, likeHits)
	}
	scored := hits[:0]
	for _, hit := range hits {
		hit.Score = score(queryTerms, hit.Text)
		if hit.Score > 0 {
			scored = append(scored, hit)
		}
	}
	sortHits(scored)
	if request.Limit > 0 && len(scored) > request.Limit {
		scored = scored[:request.Limit]
	}
	return scored, nil
}

func hasEnoughExactMatches(hits []Hit, queryTerms []string, required int) bool {
	exact := 0
	for _, hit := range hits {
		if score(queryTerms, hit.Text) == 1 {
			exact++
			if exact >= required {
				return true
			}
		}
	}
	return false
}

func queryCandidates(ctx context.Context, db *sql.DB, statement string, ranked bool, args ...any) ([]Hit, error) {
	rows, err := db.QueryContext(ctx, statement, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var hits []Hit
	for rows.Next() {
		var rank sql.NullFloat64
		var rankTarget *sql.NullFloat64
		if ranked {
			rankTarget = &rank
		}
		hit, err := scanHit(rows, rankTarget)
		if err != nil {
			return nil, err
		}
		if rank.Valid {
			rawScore := -rank.Float64
			hit.RawScore = &rawScore
			hit.RawScoreKind = RawScoreKind
		}
		hits = append(hits, hit)
	}
	return hits, rows.Err()
}

func mergeHits(primary, secondary []Hit) []Hit {
	seen := make(map[string]struct{}, len(primary)+len(secondary))
	merged := make([]Hit, 0, len(primary)+len(secondary))
	for _, group := range [][]Hit{primary, secondary} {
		for _, hit := range group {
			if _, ok := seen[hit.ID]; ok {
				continue
			}
			seen[hit.ID] = struct{}{}
			merged = append(merged, hit)
		}
	}
	return merged
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanHit(rows rowScanner, rank *sql.NullFloat64) (Hit, error) {
	var hit Hit
	var metadataJSON, createdAt, expiresAt string
	dest := []any{&hit.ID, &hit.Text, &hit.Source, &metadataJSON, &createdAt, &hit.Tier, &expiresAt}
	if rank != nil {
		dest = append(dest, rank)
	}
	if err := rows.Scan(dest...); err != nil {
		return Hit{}, err
	}
	if strings.TrimSpace(metadataJSON) != "" && metadataJSON != "{}" {
		if err := json.Unmarshal([]byte(metadataJSON), &hit.Metadata); err != nil {
			return Hit{}, err
		}
	}
	created, err := time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		return Hit{}, fmt.Errorf("sqlite memory %q created_at: %w", hit.ID, err)
	}
	hit.CreatedAt = created
	if strings.TrimSpace(expiresAt) != "" {
		expires, err := time.Parse(time.RFC3339Nano, expiresAt)
		if err != nil {
			return Hit{}, fmt.Errorf("sqlite memory %q expires_at: %w", hit.ID, err)
		}
		hit.ExpiresAt = &expires
	}
	return hit, nil
}

func filterClause(request Request, alias string) (string, []any) {
	prefix := ""
	if alias != "" {
		prefix = alias + "."
	}
	clauses := []string{"(" + prefix + "expires_at = '' OR " + prefix + "expires_at > ?)"}
	args := []any{time.Now().UTC().Format(time.RFC3339Nano)}
	if len(request.Tiers) > 0 {
		placeholders := make([]string, 0, len(request.Tiers))
		for _, tier := range request.Tiers {
			placeholders = append(placeholders, "?")
			args = append(args, tier)
		}
		clauses = append(clauses, prefix+"tier IN ("+strings.Join(placeholders, ", ")+")")
	}
	if request.Workspace != "" {
		clauses = append(clauses, "COALESCE(json_extract("+prefix+"metadata_json, '$.workspace'), '') IN ('', ?)")
		args = append(args, request.Workspace)
	}
	return strings.Join(clauses, " AND "), args
}

func likeClause(terms analysis, alias string) (string, []any) {
	prefix := ""
	if alias != "" {
		prefix = alias + "."
	}
	clauses := make([]string, 0, len(terms))
	args := make([]any, 0, len(terms))
	for _, term := range terms {
		variants := make([]string, 0, len(term.patterns))
		for _, pattern := range term.patterns {
			variants = append(variants, "lower("+prefix+"text) LIKE ? ESCAPE '\\'")
			args = append(args, "%"+escapeLike(pattern)+"%")
		}
		clauses = append(clauses, "("+strings.Join(variants, " OR ")+")")
	}
	return strings.Join(clauses, " AND "), args
}

func escapeLike(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, `%`, `\%`)
	return strings.ReplaceAll(value, `_`, `\_`)
}

func matchQuery(terms []string) string {
	unique := make([]string, 0, len(terms))
	seen := make(map[string]struct{}, len(terms))
	for _, term := range terms {
		if _, ok := seen[term]; ok {
			continue
		}
		seen[term] = struct{}{}
		unique = append(unique, `"`+strings.ReplaceAll(term, `"`, `""`)+`"`)
	}
	return strings.Join(unique, " OR ")
}

func normalizeTerms(text string) []string {
	return analyze(text).canonicalTerms()
}

func score(queryTerms []string, text string) float64 {
	if len(queryTerms) == 0 {
		return 0.1
	}
	textTerms := normalizeTerms(text)
	if len(textTerms) == 0 {
		return 0
	}
	if strings.Contains(" "+strings.Join(textTerms, " ")+" ", " "+strings.Join(queryTerms, " ")+" ") {
		return 1
	}
	textSet := make(map[string]struct{}, len(textTerms))
	for _, term := range textTerms {
		textSet[term] = struct{}{}
	}
	seenQuery := make(map[string]struct{}, len(queryTerms))
	matched := 0
	lowerText := strings.ToLower(text)
	for _, term := range queryTerms {
		if _, seen := seenQuery[term]; seen {
			continue
		}
		seenQuery[term] = struct{}{}
		if _, ok := textSet[term]; ok || strings.Contains(lowerText, term) {
			matched++
		}
	}
	if matched == 0 {
		return 0
	}
	return float64(matched) / float64(len(seenQuery))
}

func sortHits(hits []Hit) {
	sort.SliceStable(hits, func(i, j int) bool {
		if hits[i].Score != hits[j].Score {
			return hits[i].Score > hits[j].Score
		}
		if rawGreater(hits[i], hits[j]) {
			return true
		}
		if rawGreater(hits[j], hits[i]) {
			return false
		}
		return hits[i].CreatedAt.After(hits[j].CreatedAt)
	})
}

func resultLimit(limit int) int {
	if limit > 0 {
		return limit
	}
	return 50
}

func candidateLimit(limit int) int {
	if limit <= 0 {
		return 50
	}
	if limit*5 > 50 {
		return limit * 5
	}
	return 50
}

func rawGreater(left, right Hit) bool {
	return left.RawScore != nil && right.RawScore != nil && *left.RawScore > *right.RawScore
}
