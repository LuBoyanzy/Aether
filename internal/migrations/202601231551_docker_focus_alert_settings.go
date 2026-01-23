// docker_focus_alert_settings 维护 Docker 关注告警设置（系统级）。
package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

func init() {
	m.Register(func(app core.App) error {
		collection := core.NewBaseCollection("docker_focus_alert_settings")
		listRule := "@request.auth.id != \"\" && system.users.id ?= @request.auth.id"
		writeRule := listRule + " && @request.auth.role = \"admin\""

		collection.ListRule = &listRule
		collection.ViewRule = &listRule
		collection.CreateRule = &writeRule
		collection.UpdateRule = &writeRule
		collection.DeleteRule = &writeRule

		minZero := 0.0
		collection.Fields.Add(&core.RelationField{
			Name:          "system",
			CollectionId:  "2hz5ncl8tizk5nx",
			Required:      true,
			MaxSelect:     1,
			CascadeDelete: true,
		})
		collection.Fields.Add(&core.BoolField{Name: "enabled"})
		collection.Fields.Add(&core.NumberField{Name: "recovery_seconds", OnlyInt: true, Min: &minZero})
		collection.Fields.Add(&core.BoolField{Name: "alert_on_no_match"})
		collection.Fields.Add(&core.AutodateField{Name: "created", OnCreate: true})
		collection.Fields.Add(&core.AutodateField{Name: "updated", OnCreate: true, OnUpdate: true})

		collection.AddIndex("idx_docker_focus_alert_settings_system", true, "system", "")

		return app.Save(collection)
	}, func(app core.App) error {
		collection, err := app.FindCollectionByNameOrId("docker_focus_alert_settings")
		if err != nil {
			return err
		}
		return app.Delete(collection)
	})
}
