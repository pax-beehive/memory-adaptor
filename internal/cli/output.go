package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/pax-beehive/paxm/internal/memory"
	"github.com/pax-beehive/paxm/internal/telemetry"
	"github.com/pax-beehive/paxm/internal/tools"
)

func writeJSON(w io.Writer, value any) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}

func writeRecallMarkdown(w io.Writer, result tools.RecallResult) {
	if len(result.Hits) == 0 {
		_, _ = fmt.Fprintln(w, "No memories found.")
		return
	}
	for i, hit := range result.Hits {
		_, _ = fmt.Fprintf(w, "### Memory %d (%s)\n", i+1, hit.Provider)
		provenance := hit.Provenance
		if provenance == (memory.Provenance{}) {
			provenance = memory.ProvenanceFromMetadata(hit.Metadata)
		}
		_, _ = fmt.Fprintf(w, "Scope: %s\n", formatMemoryScope(provenance))
		if provenance.UserID != "" {
			_, _ = fmt.Fprintf(w, "User: %s\n", provenance.UserID)
		}
		if provenance.AgentID != "" {
			_, _ = fmt.Fprintf(w, "Agent: %s\n", provenance.AgentID)
		}
		_, _ = fmt.Fprintf(w, "Score: %.4f\n", hit.Score)
		_, _ = fmt.Fprintf(w, "Relevance: %.4f\n", hit.Relevance)
		if hit.RawScore != nil {
			if hit.RawScoreKind != "" {
				_, _ = fmt.Fprintf(w, "Raw score: %.4f (%s)\n", *hit.RawScore, hit.RawScoreKind)
			} else {
				_, _ = fmt.Fprintf(w, "Raw score: %.4f\n", *hit.RawScore)
			}
		}
		if hit.Source != "" {
			_, _ = fmt.Fprintf(w, "Source: %s\n\n", hit.Source)
		} else {
			_, _ = fmt.Fprintln(w)
		}
		_, _ = fmt.Fprintln(w, strings.TrimSpace(hit.Text))
		_, _ = fmt.Fprintln(w)
	}
}

func formatMemoryScope(provenance memory.Provenance) string {
	if provenance.ScopeType == "" || provenance.ScopeID == "" {
		return "unknown"
	}
	return provenance.ScopeType + ":" + provenance.ScopeID
}

func writeRecallContextMarkdown(w io.Writer, result tools.RecallResult, mode string) {
	if len(result.Hits) == 0 {
		writeRecallMarkdown(w, result)
		return
	}
	var context bytes.Buffer
	writeRecallMarkdown(&context, result)
	_, _ = fmt.Fprintln(w, tools.WrapRecallContext(mode, context.String()))
}

func writeHistorySummary(w io.Writer, summary telemetry.HistorySummary) {
	_, _ = fmt.Fprintln(w, "== paxm history ==")
	_, _ = fmt.Fprintf(w, "window: last %d days\n", summary.Days)
	if summary.Totals.Events == 0 {
		_, _ = fmt.Fprintln(w, "status: quiet")
		_, _ = fmt.Fprintln(w)
		_, _ = fmt.Fprintln(w, "no telemetry events recorded yet")
		writeHistoryTable(w, "storage", []string{"file", "path"}, [][]string{
			{"events", summary.Storage.EventsFile},
			{"metrics", summary.Storage.MetricsFile},
		})
		return
	}
	_, _ = fmt.Fprintf(w, "status: %s\n", historyStatus(summary.Totals))
	writeHistoryTable(w, "overview", []string{"metric", "value"}, [][]string{
		{"events", formatInt(summary.Totals.Events)},
		{"successes", formatInt(summary.Totals.Successes)},
		{"errors", formatInt(summary.Totals.Errors)},
		{"skipped", formatInt(summary.Totals.Skipped)},
		{"provider_errors", formatInt(summary.Totals.ProviderErrors)},
	})
	writeHistoryTable(w, "recall funnel", []string{"recalls", "hits", "inserted", "insert_rate"}, [][]string{{
		formatInt(summary.Totals.Recalls),
		formatInt(summary.Totals.Hits),
		formatInt(summary.Totals.Inserted),
		formatPercent(summary.Totals.Inserted, summary.Totals.Hits),
	}})
	providerWrites := sumNamedCounters(summary.Providers, func(counter telemetry.Counter) int { return counter.Writes })
	providerRefs := sumNamedCounters(summary.Providers, func(counter telemetry.Counter) int { return counter.Refs })
	writeHistoryTable(w, "write pipeline", []string{"write_events", "items", "provider_writes", "provider_refs", "flushes", "provider_ref_rate"}, [][]string{{
		formatInt(summary.Totals.Writes),
		formatInt(summary.Totals.Items),
		formatInt(providerWrites),
		formatInt(providerRefs),
		formatInt(summary.Totals.Flushes),
		formatPercent(providerRefs, providerWrites),
	}})
	writeHistoryTable(w, "storage", []string{"events_bytes", "total_bytes", "max_bytes", "files"}, [][]string{{
		formatInt64(summary.Storage.EventBytes),
		formatInt64(summary.Storage.TotalBytes),
		formatInt64(summary.Storage.MaxBytes),
		formatInt(summary.Storage.MaxFiles),
	}})
	if len(summary.Daily) > 0 {
		rows := make([][]string, 0, len(summary.Daily))
		for _, day := range summary.Daily {
			rows = append(rows, []string{
				day.Date,
				formatInt(day.Counter.Recalls),
				formatInt(day.Counter.Hits),
				formatInt(day.Counter.Inserted),
				formatInt(day.Counter.Writes),
				formatInt(day.Counter.Errors),
			})
		}
		writeHistoryTable(w, "by day", []string{"date", "recalls", "hits", "inserted", "writes", "errors"}, rows)
	}
	writeNamedCounters(w, "by profile", []string{"profile", "recalls", "hits", "inserted", "writes", "errors"}, summary.Profiles, func(counter telemetry.Counter) []string {
		return []string{formatInt(counter.Recalls), formatInt(counter.Hits), formatInt(counter.Inserted), formatInt(counter.Writes), formatInt(counter.Errors)}
	})
	writeNamedCounters(w, "by agent", []string{"agent", "passive_recalls", "passive_writes", "inserted", "flushes", "errors"}, summary.Agents, func(counter telemetry.Counter) []string {
		return []string{formatInt(counter.Recalls), formatInt(counter.Writes), formatInt(counter.Inserted), formatInt(counter.Flushes), formatInt(counter.Errors)}
	})
	writeNamedCounters(w, "by hook", []string{"hook", "recalls", "inserted", "writes", "flushes", "errors"}, summary.HookEvents, func(counter telemetry.Counter) []string {
		return []string{formatInt(counter.Recalls), formatInt(counter.Inserted), formatInt(counter.Writes), formatInt(counter.Flushes), formatInt(counter.Errors)}
	})
	writeNamedCounters(w, "by provider", []string{"provider", "recalls", "hits", "writes", "refs", "avg_write", "avg_passive_latency", "provider_errors"}, summary.Providers, func(counter telemetry.Counter) []string {
		return []string{formatInt(counter.Recalls), formatInt(counter.Hits), formatInt(counter.Writes), formatInt(counter.Refs), formatAverageMS(counter.ProviderWriteDurationMS, counter.ProviderWriteSamples), formatAverageMS(counter.PassiveWriteLatencyTotalMS, counter.PassiveWriteSamples), formatInt(counter.ProviderErrors)}
	})
}

func writeLogEvent(w io.Writer, event telemetry.Event) {
	status := "OK"
	if event.Skipped {
		status = "SKIP"
	} else if !event.Success {
		status = "ERROR"
	}
	kind := firstNonEmpty(event.Kind, "event")
	parts := []string{event.Time.UTC().Format(time.RFC3339), status, kind}
	for _, field := range []struct {
		name  string
		value string
	}{
		{name: "command", value: event.Command},
		{name: "source", value: event.Source},
		{name: "target", value: event.Target},
		{name: "hook_event", value: event.HookEvent},
		{name: "profile", value: event.Profile},
		{name: "episode_id", value: event.EpisodeID},
		{name: "session_key", value: event.SessionKey},
		{name: "provider", value: event.Provider},
	} {
		if strings.TrimSpace(field.value) != "" {
			parts = append(parts, field.name+"="+formatLogValue(field.value))
		}
	}
	for _, field := range []struct {
		name  string
		value int
	}{
		{name: "hits", value: event.HitCount},
		{name: "inserted", value: event.InsertedCount},
		{name: "items", value: event.ItemCount},
		{name: "refs", value: event.RefCount},
		{name: "flushed", value: event.Flushed},
		{name: "provider_errors", value: len(event.ProviderErrorDetails)},
	} {
		if field.value > 0 {
			parts = append(parts, field.name+"="+strconv.Itoa(field.value))
		}
	}
	if event.DurationMS > 0 {
		parts = append(parts, "duration_ms="+strconv.FormatInt(event.DurationMS, 10))
	}
	if event.ProviderDurationMS > 0 {
		parts = append(parts, "provider_duration_ms="+strconv.FormatInt(event.ProviderDurationMS, 10))
	}
	if event.PassiveWriteLatencyTotalMS > 0 {
		parts = append(parts, "passive_write_latency_total_ms="+strconv.FormatInt(event.PassiveWriteLatencyTotalMS, 10))
	}
	if event.Error != "" {
		parts = append(parts, "error="+strconv.Quote(event.Error))
	}
	_, _ = fmt.Fprintln(w, strings.Join(parts, " "))
}

func formatLogValue(value string) string {
	if strings.ContainsAny(value, " \t\r\n\"") {
		return strconv.Quote(value)
	}
	return value
}

func writeNamedCounters(w io.Writer, title string, headers []string, counters []telemetry.NamedCounter, values func(telemetry.Counter) []string) {
	if len(counters) == 0 {
		return
	}
	rows := make([][]string, 0, len(counters))
	for _, counter := range counters {
		rows = append(rows, append([]string{counter.Name}, values(counter.Counter)...))
	}
	writeHistoryTable(w, title, headers, rows)
}

func writeHistoryTable(w io.Writer, title string, headers []string, rows [][]string) {
	if len(rows) == 0 {
		return
	}
	_, _ = fmt.Fprintln(w)
	_, _ = fmt.Fprintln(w, title)
	widths := make([]int, len(headers))
	for i, header := range headers {
		widths[i] = len(header)
	}
	for _, row := range rows {
		for i := range headers {
			if i < len(row) && len(row[i]) > widths[i] {
				widths[i] = len(row[i])
			}
		}
	}
	writeHistoryRow(w, headers, widths)
	separator := make([]string, len(headers))
	for i, width := range widths {
		separator[i] = strings.Repeat("-", width)
	}
	writeHistoryRow(w, separator, widths)
	for _, row := range rows {
		writeHistoryRow(w, row, widths)
	}
}

func writeHistoryRow(w io.Writer, row []string, widths []int) {
	for i, width := range widths {
		value := ""
		if i < len(row) {
			value = row[i]
		}
		if i == 0 {
			_, _ = fmt.Fprintf(w, "  %-*s", width, value)
			continue
		}
		_, _ = fmt.Fprintf(w, "  %*s", width, value)
	}
	_, _ = fmt.Fprintln(w)
}

func historyStatus(counter telemetry.Counter) string {
	if counter.Errors > 0 || counter.ProviderErrors > 0 {
		return "attention"
	}
	if counter.Skipped > 0 {
		return "partial"
	}
	return "ok"
}

func sumNamedCounters(counters []telemetry.NamedCounter, value func(telemetry.Counter) int) int {
	total := 0
	for _, counter := range counters {
		total += value(counter.Counter)
	}
	return total
}

func formatPercent(numerator, denominator int) string {
	if denominator == 0 {
		return "n/a"
	}
	return fmt.Sprintf("%.1f%%", float64(numerator)*100/float64(denominator))
}

func formatInt(value int) string {
	return strconv.Itoa(value)
}

func formatInt64(value int64) string {
	return strconv.FormatInt(value, 10)
}

func formatAverageMS(total int64, samples int) string {
	if samples == 0 {
		return "n/a"
	}
	return fmt.Sprintf("%.1fms", float64(total)/float64(samples))
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
