package forgejo

import "strings"

// isDraftTitle reports whether title starts with a WIP prefix,
// compared case-insensitively.
// The "[wip]" prefix matches only when followed by end-of-string
// or whitespace; "wip:" matches with only a colon terminator.
func isDraftTitle(title string) bool {
	lower := strings.ToLower(title)

	// Check "wip:" prefix — matches only if followed by colon.
	if strings.HasPrefix(lower, "wip:") {
		return true
	}

	// Check "[wip]" prefix — matches only if followed by
	// end-of-string or whitespace.
	if strings.HasPrefix(lower, "[wip]") {
		if len(lower) == 5 || (len(lower) > 5 && lower[5] == ' ') {
			return true
		}
	}

	return false
}

// addDraftPrefix prepends "WIP: " to title if it is not already a draft title.
func addDraftPrefix(title string) string {
	if isDraftTitle(title) {
		return title
	}
	return "WIP: " + title
}

// stripDraftPrefix removes a leading WIP prefix and surrounding whitespace
// from title. If title is not a draft title, it is returned unchanged.
// The "[wip]" prefix is removed only when followed by end-of-string
// or whitespace; "wip:" is removed when preceded only by the colon.
func stripDraftPrefix(title string) string {
	lower := strings.ToLower(title)

	// Check "wip:" prefix.
	if strings.HasPrefix(lower, "wip:") {
		return strings.TrimSpace(title[4:])
	}

	// Check "[wip]" prefix — only remove if followed by
	// end-of-string or whitespace.
	if strings.HasPrefix(lower, "[wip]") {
		if len(lower) == 5 || (len(lower) > 5 && lower[5] == ' ') {
			return strings.TrimSpace(title[5:])
		}
	}

	return title
}
