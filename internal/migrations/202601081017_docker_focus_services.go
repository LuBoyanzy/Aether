// docker_focus_services 维护系统级 Docker 关注服务规则集合。
// 用于容器列表的关注过滤与权限控制。
package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

func init() {
	m.Register(func(app core.App) error {
		collection := core.NewBaseCollection("docker_focus_services")
		listRule := "@request.auth.id != \"\" && system.users.id ?= @request.auth.id"
		writeRule := listRule + " && @request.auth.role != \"readonly\""

		collection.ListRule = &listRule
		collection.ViewRule = &listRule
		collection.CreateRule = &writeRule
		collection.UpdateRule = &writeRule
		collection.DeleteRule = &writeRule

		collection.Fields.Add(&core.RelationField{
			Name:          "system",
			CollectionId:  "2hz5ncl8tizk5nx",
			Required:      true,
			MaxSelect:     1,
			CascadeDelete: true,
		})
		collection.Fields.Add(&core.SelectField{
			Name:      "match_type",
			Required:  true,
			MaxSelect: 1,
			Values: []string{
				"container_name",
				"image",
				"compose_project",
				"compose_service",
				"label",
			},
		})
		collection.Fields.Add(&core.TextField{Name: "value", Required: true})
		collection.Fields.Add(&core.TextField{Name: "value2"})
		collection.Fields.Add(&core.AutodateField{Name: "created", OnCreate: true})
		collection.Fields.Add(&core.AutodateField{Name: "updated", OnCreate: true, OnUpdate: true})
		collection.AddIndex("idx_docker_focus_services_system", false, "system,match_type", "")

		return app.Save(collection)
	}, func(app core.App) error {
		collection, err := app.FindCollectionByNameOrId("docker_focus_services")
		if err != nil {
			return err
		}
		return app.Delete(collection)
	})
}
