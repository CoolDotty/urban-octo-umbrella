package main

import (
	"log"

	"github.com/pocketbase/pocketbase"
	"github.com/pocketbase/pocketbase/core"

	_ "urban-octo-umbrella/server/migrations"
)

func main() {
	app := pocketbase.New()

	app.OnServe().BindFunc(func(e *core.ServeEvent) error {
		bindAuthCookieMiddleware(e.Router)

		assets, err := loadAssets()
		if err != nil {
			return err
		}

		registerAuthRoutes(e.Router, app)
		registerStaticRoutes(e.Router, assets)

		return e.Next()
	})

	if err := app.Start(); err != nil {
		log.Fatal(err)
	}
}
