// docker_focus_alerts 增加恢复详情快照字段。
package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

func init() {
	m.Register(func(app core.App) error {
		collection, err := app.FindCollectionByNameOrId("docker_focus_alerts")
		if err != nil {
			return err
		}

		collection.Fields.Add(&core.JSONField{Name: "last_down_containers"})
		collection.Fields.Add(&core.BoolField{Name: "last_no_match"})

		return app.Save(collection)
	}, func(app core.App) error {
		collection, err := app.FindCollectionByNameOrId("docker_focus_alerts")
		if err != nil {
			return err
		}

		collection.Fields.RemoveByName("last_down_containers")
		collection.Fields.RemoveByName("last_no_match")

		return app.Save(collection)
	})
}
