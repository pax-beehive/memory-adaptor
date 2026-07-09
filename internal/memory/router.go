package memory

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
)

type ProviderBinding struct {
	Provider Provider
	Read     bool
	Write    bool
	Required bool
	Weight   float64
}

type SearchResult struct {
	Hits           []MemoryHit     `json:"hits"`
	ProviderErrors []ProviderError `json:"provider_errors,omitempty"`
}

type PutResult struct {
	Refs           []MemoryRef     `json:"refs"`
	ProviderErrors []ProviderError `json:"provider_errors,omitempty"`
}

type Router struct {
	providers []ProviderBinding
}

func NewRouter(providers []ProviderBinding) (*Router, error) {
	for _, binding := range providers {
		if binding.Provider == nil {
			return nil, errors.New("memory router provider is nil")
		}
	}
	return &Router{providers: append([]ProviderBinding(nil), providers...)}, nil
}

func (r *Router) Search(ctx context.Context, query SearchQuery) (SearchResult, error) {
	var readable []ProviderBinding
	for _, binding := range r.providers {
		if binding.Read {
			readable = append(readable, binding)
		}
	}
	if len(readable) == 0 {
		return SearchResult{}, errors.New("no readable memory providers are enabled")
	}

	type response struct {
		binding ProviderBinding
		hits    []MemoryHit
		err     error
	}

	responses := make(chan response, len(readable))
	var wg sync.WaitGroup
	for _, binding := range readable {
		wg.Add(1)
		go func(binding ProviderBinding) {
			defer wg.Done()
			hits, err := binding.Provider.Search(ctx, query)
			responses <- response{binding: binding, hits: hits, err: err}
		}(binding)
	}
	wg.Wait()
	close(responses)

	var result SearchResult
	var requiredErrs []error
	seen := make(map[string]struct{})
	for res := range responses {
		name := res.binding.Provider.Name()
		if res.err != nil {
			providerErr := ProviderError{
				Provider: name,
				Required: res.binding.Required,
				Op:       "search",
				Error:    res.err.Error(),
			}
			result.ProviderErrors = append(result.ProviderErrors, providerErr)
			if res.binding.Required {
				requiredErrs = append(requiredErrs, fmt.Errorf("%s: %w", name, res.err))
			}
			continue
		}
		weight := res.binding.Weight
		if weight == 0 {
			weight = 1
		}
		for _, hit := range res.hits {
			hit.Provider = name
			hit.Score = hit.Score * weight
			key := dedupeKey(hit)
			if _, exists := seen[key]; exists {
				continue
			}
			seen[key] = struct{}{}
			result.Hits = append(result.Hits, hit)
		}
	}
	if len(requiredErrs) > 0 {
		return result, errors.Join(requiredErrs...)
	}

	sort.SliceStable(result.Hits, func(i, j int) bool {
		if result.Hits[i].Score == result.Hits[j].Score {
			return result.Hits[i].CreatedAt.After(result.Hits[j].CreatedAt)
		}
		return result.Hits[i].Score > result.Hits[j].Score
	})
	if query.Limit > 0 && len(result.Hits) > query.Limit {
		result.Hits = result.Hits[:query.Limit]
	}
	return result, nil
}

func (r *Router) Put(ctx context.Context, item MemoryItem) (PutResult, error) {
	var writable []ProviderBinding
	for _, binding := range r.providers {
		if binding.Write {
			writable = append(writable, binding)
		}
	}
	if len(writable) == 0 {
		return PutResult{}, errors.New("no writable memory providers are enabled")
	}

	type response struct {
		binding ProviderBinding
		ref     MemoryRef
		err     error
	}

	responses := make(chan response, len(writable))
	var wg sync.WaitGroup
	for _, binding := range writable {
		wg.Add(1)
		go func(binding ProviderBinding) {
			defer wg.Done()
			ref, err := binding.Provider.Put(ctx, item)
			responses <- response{binding: binding, ref: ref, err: err}
		}(binding)
	}
	wg.Wait()
	close(responses)

	var result PutResult
	var requiredErrs []error
	for res := range responses {
		name := res.binding.Provider.Name()
		if res.err != nil {
			providerErr := ProviderError{
				Provider: name,
				Required: res.binding.Required,
				Op:       "put",
				Error:    res.err.Error(),
			}
			result.ProviderErrors = append(result.ProviderErrors, providerErr)
			if res.binding.Required {
				requiredErrs = append(requiredErrs, fmt.Errorf("%s: %w", name, res.err))
			}
			continue
		}
		res.ref.Provider = name
		result.Refs = append(result.Refs, res.ref)
	}
	if len(requiredErrs) > 0 {
		return result, errors.Join(requiredErrs...)
	}
	sort.SliceStable(result.Refs, func(i, j int) bool {
		return result.Refs[i].Provider < result.Refs[j].Provider
	})
	return result, nil
}

func (r *Router) Health(ctx context.Context) ([]ProviderHealth, error) {
	if len(r.providers) == 0 {
		return nil, errors.New("no memory providers are enabled")
	}
	statuses := make([]ProviderHealth, 0, len(r.providers))
	var requiredErrs []error
	for _, binding := range r.providers {
		name := binding.Provider.Name()
		err := binding.Provider.Health(ctx)
		status := ProviderHealth{
			Provider: name,
			Required: binding.Required,
			OK:       err == nil,
		}
		if err != nil {
			status.Error = err.Error()
			if binding.Required {
				requiredErrs = append(requiredErrs, fmt.Errorf("%s: %w", name, err))
			}
		}
		statuses = append(statuses, status)
	}
	if len(requiredErrs) > 0 {
		return statuses, errors.Join(requiredErrs...)
	}
	return statuses, nil
}

func dedupeKey(hit MemoryHit) string {
	if hit.Text != "" {
		return "text:" + strings.Join(strings.Fields(strings.ToLower(hit.Text)), " ")
	}
	return "id:" + hit.Provider + ":" + hit.ID
}
