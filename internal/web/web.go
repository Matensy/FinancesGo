// Package web embeds the templates and static assets.
package web

import "embed"

// Templates holds the HTML templates.
//
//go:embed templates/*.html templates/partials/*.html
var Templates embed.FS

// Static holds the CSS/JS/vendor assets served under /static/.
//
//go:embed static
var Static embed.FS
