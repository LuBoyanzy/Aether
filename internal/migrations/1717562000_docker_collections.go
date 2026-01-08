package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

func init() {
	m.Register(func(app core.App) error {
		if err := createDockerAuditsCollection(app); err != nil {
			return err
		}
		if err := createDockerRegistriesCollection(app); err != nil {
			return err
		}
		if err := createDockerComposeTemplatesCollection(app); err != nil {
			return err
		}
		return nil
	}, func(app core.App) error {
		if err := deleteCollection(app, "docker_compose_templates"); err != nil {
			return err
		}
		if err := deleteCollection(app, "docker_registries"); err != nil {
			return err
		}
		if err := deleteCollection(app, "docker_audits"); err != nil {
			return err
		}
		return nil
	})
}

func createDockerAuditsCollection(app core.App) error {
	collection := core.NewBaseCollection("docker_audits")
	listRule := "@request.auth.id != \"\" && (user.id = @request.auth.id || system.users.id ?= @request.auth.id)"

	collection.ListRule = &listRule
	collection.ViewRule = &listRule
	collection.CreateRule = nil
	collection.UpdateRule = nil
	collection.DeleteRule = nil

	collection.Fields.Add(&core.RelationField{
		Name:          "system",
		CollectionId:  "2hz5ncl8tizk5nx",
		Required:      false,
		MaxSelect:     1,
		CascadeDelete: true,
	})
	collection.Fields.Add(&core.RelationField{
		Name:          "user",
		CollectionId:  "_pb_users_auth_",
		Required:      true,
		MaxSelect:     1,
		CascadeDelete: true,
	})
	collection.Fields.Add(&core.TextField{Name: "action", Required: true})
	collection.Fields.Add(&core.TextField{Name: "resource_type", Required: true})
	collection.Fields.Add(&core.TextField{Name: "resource_id"})
	collection.Fields.Add(&core.TextField{Name: "status", Required: true})
	collection.Fields.Add(&core.TextField{Name: "detail"})
	collection.Fields.Add(&core.AutodateField{Name: "created", OnCreate: true})
	collection.Fields.Add(&core.AutodateField{Name: "updated", OnCreate: true, OnUpdate: true})

	collection.AddIndex("idx_docker_audits_system_created", false, "system,created", "")

	return app.Save(collection)
}

func createDockerRegistriesCollection(app core.App) error {
	collection := core.NewBaseCollection("docker_registries")
	listRule := "@request.auth.id != \"\" && created_by.id = @request.auth.id"
	writeRule := listRule + " && @request.auth.role != \"readonly\""

	collection.ListRule = &listRule
	collection.ViewRule = &listRule
	collection.CreateRule = &writeRule
	collection.UpdateRule = &writeRule
	collection.DeleteRule = &writeRule

	collection.Fields.Add(&core.TextField{Name: "name", Required: true})
	collection.Fields.Add(&core.TextField{Name: "server", Required: true})
	collection.Fields.Add(&core.TextField{Name: "username"})
	collection.Fields.Add(&core.TextField{Name: "password", Hidden: true})
	collection.Fields.Add(&core.RelationField{
		Name:          "created_by",
		CollectionId:  "_pb_users_auth_",
		Required:      true,
		MaxSelect:     1,
		CascadeDelete: true,
	})
	collection.Fields.Add(&core.AutodateField{Name: "created", OnCreate: true})
	collection.Fields.Add(&core.AutodateField{Name: "updated", OnCreate: true, OnUpdate: true})

	collection.AddIndex("idx_docker_registries_created_by", false, "created_by", "")

	return app.Save(collection)
}

func createDockerComposeTemplatesCollection(app core.App) error {
	collection := core.NewBaseCollection("docker_compose_templates")
	listRule := "@request.auth.id != \"\" && created_by.id = @request.auth.id"
	writeRule := listRule + " && @request.auth.role != \"readonly\""

	collection.ListRule = &listRule
	collection.ViewRule = &listRule
	collection.CreateRule = &writeRule
	collection.UpdateRule = &writeRule
	collection.DeleteRule = &writeRule

	collection.Fields.Add(&core.TextField{Name: "name", Required: true})
	collection.Fields.Add(&core.TextField{Name: "description"})
	collection.Fields.Add(&core.TextField{Name: "content", Required: true})
	collection.Fields.Add(&core.TextField{Name: "env"})
	collection.Fields.Add(&core.RelationField{
		Name:          "created_by",
		CollectionId:  "_pb_users_auth_",
		Required:      true,
		MaxSelect:     1,
		CascadeDelete: true,
	})
	collection.Fields.Add(&core.AutodateField{Name: "created", OnCreate: true})
	collection.Fields.Add(&core.AutodateField{Name: "updated", OnCreate: true, OnUpdate: true})

	collection.AddIndex("idx_docker_templates_created_by", false, "created_by", "")

	return app.Save(collection)
}

func deleteCollection(app core.App, name string) error {
	collection, err := app.FindCollectionByNameOrId(name)
	if err != nil {
		return err
	}
	return app.Delete(collection)
}
