package forgejo

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"path"
	"strings"

	"go.abhg.dev/gs/internal/forge"
	forgejogw "go.abhg.dev/gs/internal/gateway/forgejo"
	"golang.org/x/sync/errgroup"
)

// ListChangeTemplates returns PR templates defined in the repository.
// It probes the candidate paths from ChangeTemplatePaths concurrently
// so that the lookup fits within the caller's timeout
// even on high-latency instances,
// and returns the first template in path-priority order.
func (r *Repository) ListChangeTemplates(
	ctx context.Context,
) ([]*forge.ChangeTemplate, error) {
	paths := r.forge.ChangeTemplatePaths()
	found := make([]*forgejogw.ContentsResponse, len(paths))

	eg, ctx := errgroup.WithContext(ctx)
	for i, p := range paths {
		eg.Go(func() error {
			contents, _, err := r.client.ContentsGet(ctx, r.owner, r.repo, p)
			if err != nil {
				if errors.Is(err, forgejogw.ErrNotFound) {
					return nil
				}
				return fmt.Errorf("get template %q: %w", p, err)
			}
			found[i] = contents
			return nil
		})
	}
	if err := eg.Wait(); err != nil {
		return nil, err
	}

	for i, contents := range found {
		if contents == nil {
			continue
		}

		// Forgejo may wrap base64 content with embedded newlines.
		// Strip them before decoding.
		encoded := strings.ReplaceAll(contents.Content, "\n", "")
		decoded, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			return nil, fmt.Errorf("decode template %q: %w", paths[i], err)
		}

		return []*forge.ChangeTemplate{
			{
				Filename: path.Base(paths[i]),
				Body:     string(decoded),
			},
		}, nil
	}

	return nil, nil
}
