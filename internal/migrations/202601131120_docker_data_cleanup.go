package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

func init() {
	m.Register(func(app core.App) error {
		if err := createDockerDataCleanupConfigsCollection(app); err != nil {
			return err
		}
		if err := createDockerDataCleanupRunsCollection(app); err != nil {
			return err
		}
		return nil
	}, func(app core.App) error {
		if err := deleteCollection(app, "docker_data_cleanup_runs"); err != nil {
			return err
		}
		if err := deleteCollection(app, "docker_data_cleanup_configs"); err != nil {
			return err
		}
		return nil
	})
}

func createDockerDataCleanupConfigsCollection(app core.App) error {
	collection := core.NewBaseCollection("docker_data_cleanup_configs")
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
	collection.Fields.Add(&core.JSONField{Name: "mysql"})
	collection.Fields.Add(&core.TextField{Name: "mysql_password", Hidden: true})
	collection.Fields.Add(&core.JSONField{Name: "redis"})
	collection.Fields.Add(&core.TextField{Name: "redis_password", Hidden: true})
	collection.Fields.Add(&core.JSONField{Name: "minio"})
	collection.Fields.Add(&core.TextField{Name: "minio_secret_key", Hidden: true})
	collection.Fields.Add(&core.JSONField{Name: "es"})
	collection.Fields.Add(&core.TextField{Name: "es_password", Hidden: true})
	collection.Fields.Add(&core.AutodateField{Name: "created", OnCreate: true})
	collection.Fields.Add(&core.AutodateField{Name: "updated", OnCreate: true, OnUpdate: true})

	collection.AddIndex("idx_docker_data_cleanup_configs_system", true, "system", "")

	return app.Save(collection)
}

func createDockerDataCleanupRunsCollection(app core.App) error {
	configCollection, err := app.FindCollectionByNameOrId("docker_data_cleanup_configs")
	if err != nil {
		return err
	}
	collection := core.NewBaseCollection("docker_data_cleanup_runs")
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
	collection.Fields.Add(&core.RelationField{
		Name:          "config",
		CollectionId:  configCollection.Id,
		Required:      true,
		MaxSelect:     1,
		CascadeDelete: true,
	})
	collection.Fields.Add(&core.TextField{Name: "status", Required: true})
	min := 0.0
	max := 100.0
	collection.Fields.Add(&core.NumberField{Name: "progress", Min: &min, Max: &max})
	collection.Fields.Add(&core.TextField{Name: "step"})
	collection.Fields.Add(&core.JSONField{Name: "logs"})
	collection.Fields.Add(&core.JSONField{Name: "results"})
	collection.Fields.Add(&core.AutodateField{Name: "created", OnCreate: true})
	collection.Fields.Add(&core.AutodateField{Name: "updated", OnCreate: true, OnUpdate: true})

	collection.AddIndex("idx_docker_data_cleanup_runs_system_created", false, "system,created", "")

	return app.Save(collection)
}
