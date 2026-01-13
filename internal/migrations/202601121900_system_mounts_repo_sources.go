package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

func init() {
	m.Register(func(app core.App) error {
		if err := createSystemNetworkMountsCollection(app); err != nil {
			return err
		}
		if err := createSystemRepoSourcesCollection(app); err != nil {
			return err
		}
		return nil
	}, func(app core.App) error {
		if err := deleteCollection(app, "system_repo_sources"); err != nil {
			return err
		}
		if err := deleteCollection(app, "system_network_mounts"); err != nil {
			return err
		}
		return nil
	})
}

func createSystemNetworkMountsCollection(app core.App) error {
	collection := core.NewBaseCollection("system_network_mounts")
	listRule := "@request.auth.id != \"\" && system.users.id ?= @request.auth.id"

	collection.ListRule = &listRule
	collection.ViewRule = &listRule
	collection.CreateRule = nil
	collection.UpdateRule = nil
	collection.DeleteRule = nil

	collection.Fields.Add(&core.RelationField{
		Name:          "system",
		CollectionId:  "2hz5ncl8tizk5nx",
		Required:      true,
		MaxSelect:     1,
		CascadeDelete: true,
	})
	collection.Fields.Add(&core.TextField{Name: "source"})
	collection.Fields.Add(&core.TextField{Name: "source_host"})
	collection.Fields.Add(&core.TextField{Name: "source_path"})
	collection.Fields.Add(&core.TextField{Name: "mount_point", Required: true})
	collection.Fields.Add(&core.TextField{Name: "fstype", Required: true})
	collection.Fields.Add(&core.NumberField{Name: "total_bytes", OnlyInt: true})
	collection.Fields.Add(&core.NumberField{Name: "used_bytes", OnlyInt: true})
	collection.Fields.Add(&core.NumberField{Name: "used_pct"})
	collection.Fields.Add(&core.NumberField{Name: "updated", OnlyInt: true})

	collection.AddIndex("idx_system_network_mounts_system", false, "system", "")

	return app.Save(collection)
}

func createSystemRepoSourcesCollection(app core.App) error {
	collection := core.NewBaseCollection("system_repo_sources")
	listRule := "@request.auth.id != \"\" && system.users.id ?= @request.auth.id"

	collection.ListRule = &listRule
	collection.ViewRule = &listRule
	collection.CreateRule = nil
	collection.UpdateRule = nil
	collection.DeleteRule = nil

	collection.Fields.Add(&core.RelationField{
		Name:          "system",
		CollectionId:  "2hz5ncl8tizk5nx",
		Required:      true,
		MaxSelect:     1,
		CascadeDelete: true,
	})
	collection.Fields.Add(&core.TextField{Name: "manager", Required: true})
	collection.Fields.Add(&core.TextField{Name: "repo_id", Required: true})
	collection.Fields.Add(&core.TextField{Name: "name"})
	collection.Fields.Add(&core.TextField{Name: "url", Required: true})
	collection.Fields.Add(&core.BoolField{Name: "enabled"})
	collection.Fields.Add(&core.TextField{Name: "status"})
	collection.Fields.Add(&core.TextField{Name: "error"})
	collection.Fields.Add(&core.NumberField{Name: "checked_at", OnlyInt: true})
	collection.Fields.Add(&core.NumberField{Name: "updated", OnlyInt: true})

	collection.AddIndex("idx_system_repo_sources_system", false, "system", "")

	return app.Save(collection)
}
