package utils

import (
	"net/url"
	"strings"
)

// PublicMediaPath strips the object-storage origin from a public media URL and
// returns the edge-relative path (e.g. "/goshop-media/products/x.png").
//
// Stored values must be origin-agnostic: the dashboard, the storefront and the
// API all render them, and the browser reaches storage through the edge, not
// through the S3/MinIO service the API itself dials. Persisting an absolute URL
// bakes in the dev host (http://localhost:9000) and breaks on any domain change.
//
// Absolute URLs that are not under the media base are returned unchanged so
// external images (CDNs, seeded fixtures) keep working.
func PublicMediaPath(mediaBaseURL, rawURL string) string {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return ""
	}
	base := strings.TrimRight(strings.TrimSpace(mediaBaseURL), "/")
	if base == "" || strings.HasPrefix(rawURL, "/") {
		return rawURL
	}

	u, err := url.Parse(rawURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
		return rawURL
	}
	b, err := url.Parse(base)
	if err != nil || b.Path == "" {
		return rawURL
	}

	// Ours, whichever host (and whichever bucket name) wrote it.
	switch {
	case strings.HasPrefix(u.Path, b.Path+"/"):
		return b.Path + strings.TrimPrefix(u.Path, b.Path)
	case strings.HasPrefix(u.Path, "/goshop-media/"):
		return u.Path
	}

	// An origin we know is storage, carrying a bare object key (bucket name
	// missing from the path). Both storage origins the app has ever produced
	// are recognised by their bucket prefix.
	if strings.Contains(u.Path, "/"+b.Path) || strings.HasPrefix(b.Path, "/") && strings.Contains(u.Path, b.Path) {
		return b.Path + u.Path
	}
	return rawURL
}
