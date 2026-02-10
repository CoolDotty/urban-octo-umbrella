//go:build !dev

package main

import (
	"embed"
	"io/fs"
)

//go:embed all:web/dist
var embeddedFiles embed.FS

func loadAssets() (*spaAssets, error) {
	fsys, err := fs.Sub(embeddedFiles, "web/dist")
	if err != nil {
		return nil, err
	}

	indexHTML, err := fs.ReadFile(fsys, "index.html")
	if err != nil {
		return nil, err
	}

	return &spaAssets{
		fs:        fsys,
		indexHTML: indexHTML,
	}, nil
}
