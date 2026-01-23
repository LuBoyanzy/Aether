// Migration adds error field to system_network_mounts.
package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

func init() {
	m.Register(func(app core.App) error {
		collection, err := app.FindCollectionByNameOrId("system_network_mounts")
		if err != nil {
			return err
		}

		collection.Fields.Add(&core.TextField{Name: "error"})

		return app.Save(collection)
	}, func(app core.App) error {
		collection, err := app.FindCollectionByNameOrId("system_network_mounts")
		if err != nil {
			return err
		}

		collection.Fields.RemoveByName("error")

		return app.Save(collection)
	})
}
