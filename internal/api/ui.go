package api

import (
	"embed"
	"io/fs"
	"net/http"
)

// webAssets is the embedded SPA (ADR-0014): index.html + app.js + style.css served from the
// binary so one artifact serves API + UI everywhere, with zero external network fetches. The
// `all:` prefix embeds every file in the tree (including any dotfiles) so the embedded FS is a
// faithful copy of web/. The directive is co-located with UIHandler because //go:embed can only
// reference the embedding file's own directory subtree — not a repo-root sibling.
//
//go:embed all:web
var webAssets embed.FS

// UIHandler serves the embedded web/ SPA at "/". GET "/" resolves to index.html (200,
// text/html via extension detection). It is a thin, stdlib-forward wrapper: fs.Sub strips the
// "web" prefix so the SPA is served from the FS root, and http.FileServerFS (Go 1.22+) does the
// content-type + index.html resolution — no bespoke file walking (ARCHITECTURE §Frontend Design
// Language; USERS §UC-1).
func UIHandler() http.Handler {
	sub, err := fs.Sub(webAssets, "web")
	if err != nil {
		// Unreachable in a correctly-built binary: the embed above guarantees web/ exists.
		// Panicking here surfaces a broken build immediately rather than serving 404s.
		panic("api: embedded web/ subtree missing: " + err.Error())
	}
	return http.FileServerFS(sub)
}
