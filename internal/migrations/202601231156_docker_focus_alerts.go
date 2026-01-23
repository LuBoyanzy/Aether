// docker_focus_alerts 记录 Docker 关注服务告警状态。
package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

func init() {
	m.Register(func(app core.App) error {
		focusCollection, err := app.FindCollectionByNameOrId("docker_focus_services")
		if err != nil {
			return err
		}
		collection := core.NewBaseCollection("docker_focus_alerts")
		listRule := "@request.auth.id != \"\" && system.users.id ?= @request.auth.id"
		writeRule := listRule + " && @request.auth.role != \"readonly\""

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
		collection.Fields.Add(&core.RelationField{
			Name:          "focus_rule",
			CollectionId:  focusCollection.Id,
			Required:      true,
			MaxSelect:     1,
			CascadeDelete: true,
		})
		collection.Fields.Add(&core.BoolField{Name: "triggered"})
		collection.Fields.Add(&core.NumberField{Name: "running_count", OnlyInt: true, Min: &minZero})
		collection.Fields.Add(&core.NumberField{Name: "total_count", OnlyInt: true, Min: &minZero})
		collection.Fields.Add(&core.DateField{Name: "recovery_since"})
		collection.Fields.Add(&core.AutodateField{Name: "created", OnCreate: true})
		collection.Fields.Add(&core.AutodateField{Name: "updated", OnCreate: true, OnUpdate: true})

		collection.AddIndex("idx_docker_focus_alerts_rule", true, "system,focus_rule", "")
		collection.AddIndex("idx_docker_focus_alerts_system", false, "system", "")

		return app.Save(collection)
	}, func(app core.App) error {
		collection, err := app.FindCollectionByNameOrId("docker_focus_alerts")
		if err != nil {
			return err
		}
		return app.Delete(collection)
	})
}
