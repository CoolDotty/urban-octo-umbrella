package main

import (
	"embed"
	"io/fs"
	"net/http"
	"strings"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"
)

//go:embed web/dist/*
var embeddedFiles embed.FS

func main() {
	app := pocketbase.New()

	app.OnServe().BindFunc(func(e *core.ServeEvent) error {
		fsys, err := fs.Sub(embeddedFiles, "web/dist")
		if err != nil {
			return err
		}

		indexHTML, err := fs.ReadFile(fsys, "index.html")
		if err != nil {
			return err
		}

		e.Router.GET("/*", func(re *core.RequestEvent) error {
			path := strings.TrimPrefix(re.Request.URL.Path, "/")
			if path == "" {
				path = "index.html"
			}

			if _, err := fs.Stat(fsys, path); err == nil {
				return re.FileFS(fsys, path)
			}

			return re.HTML(http.StatusOK, string(indexHTML))
		})

		return e.Next()
	})

	if err := app.Start(); err != nil {
		panic(err)
	}
}
