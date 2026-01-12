package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

func init() {
	m.Register(func(app core.App) error {
		collection := core.NewBaseCollection("docker_service_configs")
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
		collection.Fields.Add(&core.TextField{Name: "name", Required: true})
		collection.Fields.Add(&core.TextField{Name: "url", Required: true})
		collection.Fields.Add(&core.TextField{Name: "token", Required: true, Hidden: true})
		collection.Fields.Add(&core.AutodateField{Name: "created", OnCreate: true})
		collection.Fields.Add(&core.AutodateField{Name: "updated", OnCreate: true, OnUpdate: true})
		collection.AddIndex("idx_docker_service_configs_system", false, "system,name", "")

		return app.Save(collection)
	}, func(app core.App) error {
		collection, err := app.FindCollectionByNameOrId("docker_service_configs")
		if err != nil {
			return err
		}
		return app.Delete(collection)
	})
}
