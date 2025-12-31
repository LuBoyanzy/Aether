package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

func init() {
	m.Register(func(app core.App) error {
		collection, err := app.FindCollectionByNameOrId("containers")
		if err != nil {
			return err
		}

		// remove deprecated health column
		collection.Fields.RemoveByName("health")

		// add uptime (seconds) column after status for table/order stability
		min := 0.0
		collection.Fields.AddAt(4, &core.NumberField{
			Name:    "uptime",
			Min:     &min,
			OnlyInt: true,
		})

		return app.Save(collection)
	}, func(app core.App) error {
		collection, err := app.FindCollectionByNameOrId("containers")
		if err != nil {
			return err
		}

		collection.Fields.RemoveByName("uptime")
		collection.Fields.AddAt(4, &core.NumberField{
			Name: "health",
		})

		return app.Save(collection)
	})
}
