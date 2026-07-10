package backfill

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/pax-beehive/memory-adaptor/internal/facade"
	"github.com/pax-beehive/memory-adaptor/internal/sessions"
)

const maxItemBytes = 24 * 1024

type Runner struct {
	Store   *Store
	Service *facade.Service
}

type RunOptions struct {
	Scope        string
	RunID        string
	Mode         string
	Agent        string
	Provider     string
	Files        []sessions.File
	Cutoff       time.Time
	RateInterval time.Duration
	Progress     func(Status)
	Started      func(Status)
}

func NewRunID() string {
	var bytes [8]byte
	if _, err := rand.Read(bytes[:]); err == nil {
		return hex.EncodeToString(bytes[:])
	}
	return strconv.FormatInt(time.Now().UnixNano(), 36)
}

func (r Runner) Run(ctx context.Context, options RunOptions) (Status, error) {
	if r.Store == nil || r.Service == nil {
		return Status{}, errors.New("backfill runner is not configured")
	}
	if options.RunID == "" {
		options.RunID = NewRunID()
	}
	release, err := r.Store.Acquire(options.Scope, options.RunID)
	if err != nil {
		return Status{}, err
	}
	defer release()

	started := time.Now().UTC()
	status := Status{
		State:      "running",
		Mode:       firstNonEmpty(options.Mode, "foreground"),
		RunID:      options.RunID,
		PID:        processID(),
		Agent:      options.Agent,
		Provider:   options.Provider,
		StartedAt:  started,
		TotalFiles: len(options.Files),
	}
	for _, file := range options.Files {
		status.TotalBytes += file.Size
	}
	if err := r.publish(options, status); err != nil {
		return status, err
	}
	if options.Started != nil {
		options.Started(status)
	}

	var operationErrors []error
	var nextUpload time.Time
	for _, file := range options.Files {
		if err := ctx.Err(); err != nil {
			status.State = "paused"
			status.FinishedAt = time.Now().UTC()
			_ = r.publish(options, status)
			return status, err
		}
		turns, readErr := sessions.ReadFile(options.Agent, file.Path, options.Cutoff)
		fileStartBytes := status.ProcessedBytes
		if readErr != nil {
			status.Failed++
			operationErrors = append(operationErrors, readErr)
		} else {
			var items []facade.IngestInput
			for _, turn := range turns {
				items = append(items, turnItems(turn)...)
			}
			status.Discovered += len(items)
			_ = r.publish(options, status)
			for index, item := range items {
				done, checkErr := r.Store.Succeeded(options.Scope, item.ID)
				if checkErr != nil {
					return status, checkErr
				}
				if done {
					status.Skipped++
					status.ProcessedBytes = fileStartBytes + file.Size*int64(index+1)/int64(len(items))
					_ = r.publish(options, status)
					continue
				}
				if wait := time.Until(nextUpload); options.RateInterval > 0 && wait > 0 {
					timer := time.NewTimer(wait)
					select {
					case <-ctx.Done():
						timer.Stop()
						status.State = "paused"
						status.FinishedAt = time.Now().UTC()
						_ = r.publish(options, status)
						return status, ctx.Err()
					case <-timer.C:
					}
				}
				result, ingestErr := r.Service.IngestBatchToProvider(ctx, options.Provider, facade.IngestBatchInput{Items: []facade.IngestInput{item}})
				nextUpload = time.Now().Add(options.RateInterval)
				if ingestErr != nil {
					status.Failed++
					operationErrors = append(operationErrors, fmt.Errorf("%s: %w", item.ID, ingestErr))
					status.ProcessedBytes = fileStartBytes + file.Size*int64(index+1)/int64(len(items))
					_ = r.publish(options, status)
					continue
				}
				providerRef := ""
				if len(result.Refs) > 0 {
					providerRef = result.Refs[0].ID
				}
				if err := r.Store.MarkSucceeded(options.Scope, item.ID, providerRef); err != nil {
					return status, err
				}
				status.Uploaded++
				status.ProcessedBytes = fileStartBytes + file.Size*int64(index+1)/int64(len(items))
				_ = r.publish(options, status)
			}
		}
		status.ProcessedFiles++
		status.ProcessedBytes = fileStartBytes + file.Size
		updateRates(&status)
		if err := r.publish(options, status); err != nil {
			return status, err
		}
	}
	status.FinishedAt = time.Now().UTC()
	status.State = "completed"
	if len(operationErrors) > 0 {
		status.State = "completed_with_errors"
		status.Error = errors.Join(operationErrors...).Error()
	}
	updateRates(&status)
	if err := r.publish(options, status); err != nil {
		return status, err
	}
	return status, errors.Join(operationErrors...)
}

func (r Runner) publish(options RunOptions, status Status) error {
	updateRates(&status)
	if err := r.Store.WriteStatus(options.Scope, status); err != nil {
		return err
	}
	if options.Progress != nil {
		options.Progress(status)
	}
	return nil
}

func updateRates(status *Status) {
	elapsed := time.Since(status.StartedAt)
	if elapsed <= 0 {
		return
	}
	status.ItemsPerSecond = float64(status.Uploaded+status.Skipped) / elapsed.Seconds()
	status.BytesPerSecond = float64(status.ProcessedBytes) / elapsed.Seconds()
	remaining := status.TotalBytes - status.ProcessedBytes
	if remaining > 0 && status.BytesPerSecond > 0 {
		status.ETASeconds = int64(float64(remaining) / status.BytesPerSecond)
	} else {
		status.ETASeconds = 0
	}
}

func turnItems(turn sessions.Turn) []facade.IngestInput {
	header := fmt.Sprintf("Historical %s agent session turn.\n\nUser:\n%s\n\nAssistant:\n", turn.Agent, turn.User)
	text := header + turn.Assistant
	parts := splitUTF8(text, maxItemBytes)
	items := make([]facade.IngestInput, 0, len(parts))
	for index, part := range parts {
		id := turn.ID
		if len(parts) > 1 {
			id += "-part-" + strconv.Itoa(index+1)
		}
		metadata := map[string]string{
			"backfill":   "true",
			"agent":      turn.Agent,
			"session_id": turn.SessionID,
			"workspace":  turn.Workspace,
		}
		if len(parts) > 1 {
			metadata["part"] = strconv.Itoa(index + 1)
			metadata["parts"] = strconv.Itoa(len(parts))
		}
		items = append(items, facade.IngestInput{
			ID:        id,
			Text:      part,
			Source:    "backfill:" + turn.Agent,
			Metadata:  metadata,
			CreatedAt: turn.CreatedAt,
		})
	}
	return items
}

func splitUTF8(value string, size int) []string {
	if len(value) <= size {
		return []string{value}
	}
	var parts []string
	for len(value) > size {
		cut := size
		for cut > 0 && !utf8.RuneStart(value[cut]) {
			cut--
		}
		if cut == 0 {
			cut = size
		}
		parts = append(parts, strings.TrimSpace(value[:cut]))
		value = value[cut:]
	}
	if strings.TrimSpace(value) != "" {
		parts = append(parts, strings.TrimSpace(value))
	}
	return parts
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func processID() int {
	return processIDValue
}
