package unwrap

import "lesiw.io/tools/internal/header"

// collapseHeaders merges every multi-line header that fits within the width
// limit back onto one line.
func collapseHeaders(r *renderer) {
	var (
		tokFile = r.tokFile
		file    = r.file
	)
	for _, s := range header.Spans(file) {
		if sameLine(tokFile, s[0], s[1]) {
			continue
		}
		if lineCommentWithin(file, s[0], s[1]) {
			continue
		}
		join(r, s[0], s[1])
	}
}
