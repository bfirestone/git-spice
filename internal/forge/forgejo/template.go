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
)

// ListChangeTemplates returns PR templates defined in the repository.
// It iterates the candidate paths from ChangeTemplatePaths,
// returning the first template found.
func (r *Repository) ListChangeTemplates(
	ctx context.Context,
) ([]*forge.ChangeTemplate, error) {
	for _, p := range r.forge.ChangeTemplatePaths() {
		contents, _, err := r.client.ContentsGet(ctx, r.owner, r.repo, p)
		if err != nil {
			if errors.Is(err, forgejogw.ErrNotFound) {
				continue
			}
			return nil, fmt.Errorf("get template %q: %w", p, err)
		}

		// Forgejo may wrap base64 content with embedded newlines.
		// Strip them before decoding.
		encoded := strings.ReplaceAll(contents.Content, "\n", "")
		decoded, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			return nil, fmt.Errorf("decode template %q: %w", p, err)
		}

		return []*forge.ChangeTemplate{
			{
				Filename: path.Base(p),
				Body:     string(decoded),
			},
		}, nil
	}

	return nil, nil
}
