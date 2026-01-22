package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

func init() {
	m.Register(func(app core.App) error {
		collection, err := app.FindCollectionByNameOrId("docker_focus_services")
		if err != nil {
			return err
		}

		collection.Fields.Add(&core.TextField{Name: "description"})

		return app.Save(collection)
	}, func(app core.App) error {
		collection, err := app.FindCollectionByNameOrId("docker_focus_services")
		if err != nil {
			return err
		}

		collection.Fields.RemoveByName("description")

		return app.Save(collection)
	})
}
