package forgejo

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	forgejogw "go.abhg.dev/gs/internal/gateway/forgejo"
)

// ensureLabels resolves each label name to a Forgejo label ID,
// creating labels that do not yet exist.
// On a creation conflict (HTTP 409), it re-lists and resolves the ID.
func (r *Repository) ensureLabels(
	ctx context.Context,
	names []string,
) ([]int64, error) {
	if len(names) == 0 {
		return nil, nil
	}

	existing, _, err := r.client.LabelList(ctx, r.owner, r.repo)
	if err != nil {
		return nil, fmt.Errorf("list labels: %w", err)
	}

	// Build a name->ID index from the initial list.
	index := make(map[string]int64, len(existing))
	for _, lbl := range existing {
		index[lbl.Name] = lbl.ID
	}

	ids := make([]int64, len(names))
	for i, name := range names {
		id, ok := index[name]
		if ok {
			ids[i] = id
			continue
		}

		// Label not found — create it.
		r.log.Info("Label does not exist, creating", "name", name)
		created, _, createErr := r.client.LabelCreate(ctx, r.owner, r.repo, name)
		if createErr == nil {
			ids[i] = created.ID
			continue
		}

		// On a conflict (concurrent create), re-list and resolve.
		var apiErr *forgejogw.APIError
		if errors.As(createErr, &apiErr) && apiErr.StatusCode == http.StatusConflict {
			r.log.Debug(
				"Label may have been created by another request, re-listing",
				"name", name,
				"error", createErr,
			)
			relisted, _, listErr := r.client.LabelList(ctx, r.owner, r.repo)
			if listErr != nil {
				return nil, fmt.Errorf("re-list labels: %w", listErr)
			}
			for _, lbl := range relisted {
				if lbl.Name == name {
					ids[i] = lbl.ID
					break
				}
			}
			if ids[i] == 0 {
				return nil, fmt.Errorf("label %q not found after conflict re-list", name)
			}
			continue
		}

		return nil, fmt.Errorf("create label %q: %w", name, createErr)
	}

	return ids, nil
}
