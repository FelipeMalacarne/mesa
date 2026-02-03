// Package web implements the embedded static file server for the web application.
package web

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var assets embed.FS

func GetPublicFS() fs.FS {
	public, err := fs.Sub(assets, "dist") // Mudou de .output/public para dist
	if err != nil {
		panic(err)
	}
	return public
}
