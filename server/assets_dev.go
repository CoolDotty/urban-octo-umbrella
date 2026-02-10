//go:build dev

package main

func loadAssets() (*spaAssets, error) {
	// In dev we rely on the Vite dev server, so static assets are optional.
	return nil, nil
}
