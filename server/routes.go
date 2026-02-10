package main

import (
	"io/fs"
	"net/http"
	"strings"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tools/router"
)

func bindAuthCookieMiddleware(router *router.Router[*core.RequestEvent]) {
	router.BindFunc(func(re *core.RequestEvent) error {
		if re.Request.Header.Get("Authorization") == "" {
			if cookie, err := re.Request.Cookie(authCookieName); err == nil && cookie.Value != "" {
				re.Request.Header.Set("Authorization", "Bearer "+cookie.Value)
			}
		}

		return re.Next()
	})
}

func registerStaticRoutes(router *router.Router[*core.RequestEvent], assets *spaAssets) {
	router.GET("/*", func(re *core.RequestEvent) error {
		path := strings.TrimPrefix(re.Request.URL.Path, "/")
		if path == "" {
			path = "index.html"
		}

		if _, err := fs.Stat(assets.fs, path); err == nil {
			return re.FileFS(assets.fs, path)
		}

		return re.HTML(http.StatusOK, string(assets.indexHTML))
	})
}
