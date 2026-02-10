package main

import "io/fs"

type spaAssets struct {
	fs        fs.FS
	indexHTML []byte
}
