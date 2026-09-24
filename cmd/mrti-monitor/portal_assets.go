package main

import (
	_ "embed"
	"net/http"
)

// Los recursos se incrustan para que Agent Core conserve la marca y las
// tipografías del portal aunque se abra en su puerto independiente.
//
//go:embed portal-assets/company-logo.svg
var portalLogo []byte

//go:embed portal-assets/favicon.svg
var portalFavicon []byte

//go:embed portal-assets/big-shoulders-display-800.woff2
var portalHeadingFont []byte

//go:embed portal-assets/ibm-plex-sans-400.woff2
var portalBodyFont []byte

func portalAsset(w http.ResponseWriter, r *http.Request) {
	var content []byte
	var contentType string

	switch r.PathValue("name") {
	case "company-logo.svg":
		content, contentType = portalLogo, "image/svg+xml"
	case "favicon.svg":
		content, contentType = portalFavicon, "image/svg+xml"
	case "big-shoulders-display-800.woff2":
		content, contentType = portalHeadingFont, "font/woff2"
	case "ibm-plex-sans-400.woff2":
		content, contentType = portalBodyFont, "font/woff2"
	default:
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "public, max-age=86400")
	w.Write(content)
}
