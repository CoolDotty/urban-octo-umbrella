package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
	"github.com/pocketbase/pocketbase/tools/types"
)

func init() {
	m.Register(func(app core.App) error {
		users, err := app.FindCollectionByNameOrId("users")
		if err != nil {
			users = core.NewAuthCollection("users")
		}

		ownerRule := "id = @request.auth.id"
		if users.ListRule == nil || *users.ListRule == "" {
			users.ListRule = types.Pointer(ownerRule)
		}
		if users.ViewRule == nil || *users.ViewRule == "" {
			users.ViewRule = types.Pointer(ownerRule)
		}
		if users.UpdateRule == nil || *users.UpdateRule == "" {
			users.UpdateRule = types.Pointer(ownerRule)
		}
		if users.DeleteRule == nil || *users.DeleteRule == "" {
			users.DeleteRule = types.Pointer(ownerRule)
		}
		if users.CreateRule == nil || *users.CreateRule == "" {
			users.CreateRule = types.Pointer(`@request.auth.role = "admin"`)
		}

		if users.Fields.GetByName("display_name") == nil {
			users.Fields.Add(&core.TextField{
				Name: "display_name",
				Max:  200,
			})
		}

		if users.Fields.GetByName("role") == nil {
			users.Fields.Add(&core.TextField{
				Name: "role",
				Max:  50,
			})
		}

		if err := app.Save(users); err != nil {
			return err
		}

		invites, err := app.FindCollectionByNameOrId("invites")
		if err != nil {
			invites = core.NewBaseCollection("invites")
		}
		adminRule := `@request.auth.role = "admin"`
		invites.ListRule = types.Pointer(adminRule)
		invites.ViewRule = types.Pointer(adminRule)
		invites.CreateRule = types.Pointer(adminRule)
		invites.UpdateRule = types.Pointer(adminRule)
		invites.DeleteRule = types.Pointer(adminRule)

		if invites.Fields.GetByName("token") == nil {
			invites.Fields.Add(&core.TextField{
				Name:     "token",
				Required: true,
			})
		}
		if invites.Fields.GetByName("expires_at") == nil {
			invites.Fields.Add(&core.DateField{
				Name: "expires_at",
			})
		}
		if invites.Fields.GetByName("used_at") == nil {
			invites.Fields.Add(&core.DateField{
				Name: "used_at",
			})
		}
		if invites.Fields.GetByName("used_by") == nil {
			invites.Fields.Add(&core.RelationField{
				Name:         "used_by",
				CollectionId: users.Id,
				MaxSelect:    1,
			})
		}
		if invites.Fields.GetByName("created_by") == nil {
			invites.Fields.Add(&core.RelationField{
				Name:         "created_by",
				CollectionId: users.Id,
				MaxSelect:    1,
			})
		}

		return app.Save(invites)
	}, func(app core.App) error {
		if invites, err := app.FindCollectionByNameOrId("invites"); err == nil {
			if err := app.Delete(invites); err != nil {
				return err
			}
		}

		return nil
	})
}
