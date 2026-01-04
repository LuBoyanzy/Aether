package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

func init() {
	m.Register(func(app core.App) error {
		collection, err := app.FindCollectionByNameOrId("system_details")
		if err != nil {
			return err
		}

		collection.Fields.Add(&core.TextField{
			Name: "cuda_version",
		})
		collection.Fields.Add(&core.TextField{
			Name: "nvidia_ctk",
		})

		return app.Save(collection)
	}, func(app core.App) error {
		collection, err := app.FindCollectionByNameOrId("system_details")
		if err != nil {
			return err
		}

		collection.Fields.RemoveByName("cuda_version")
		collection.Fields.RemoveByName("nvidia_ctk")

		return app.Save(collection)
	})
}
