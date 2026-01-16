// api_tests 用于接口监控的合集、用例、调度与执行记录集合。
// 提供接口管理数据持久化与索引支持。
package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

func init() {
	m.Register(func(app core.App) error {
		if err := createApiTestCollectionsCollection(app); err != nil {
			return err
		}
		if err := createApiTestCasesCollection(app); err != nil {
			return err
		}
		if err := createApiTestScheduleConfigCollection(app); err != nil {
			return err
		}
		if err := createApiTestRunsCollection(app); err != nil {
			return err
		}
		return nil
	}, func(app core.App) error {
		if err := deleteCollection(app, "api_test_runs"); err != nil {
			return err
		}
		if err := deleteCollection(app, "api_test_schedule_config"); err != nil {
			return err
		}
		if err := deleteCollection(app, "api_test_cases"); err != nil {
			return err
		}
		if err := deleteCollection(app, "api_test_collections"); err != nil {
			return err
		}
		return nil
	})
}

func createApiTestCollectionsCollection(app core.App) error {
	collection := core.NewBaseCollection("api_test_collections")
	authRule := "@request.auth.id != \"\""

	collection.ListRule = &authRule
	collection.ViewRule = &authRule
	collection.CreateRule = &authRule
	collection.UpdateRule = &authRule
	collection.DeleteRule = &authRule

	minZero := 0.0
	collection.Fields.Add(&core.TextField{Name: "name", Required: true})
	collection.Fields.Add(&core.TextField{Name: "description"})
	collection.Fields.Add(&core.TextField{Name: "base_url"})
	collection.Fields.Add(&core.NumberField{Name: "sort_order", OnlyInt: true, Min: &minZero})
	collection.Fields.Add(&core.JSONField{Name: "tags"})
	collection.Fields.Add(&core.AutodateField{Name: "created", OnCreate: true})
	collection.Fields.Add(&core.AutodateField{Name: "updated", OnCreate: true, OnUpdate: true})

	collection.AddIndex("idx_api_test_collections_order", false, "sort_order,created", "")

	return app.Save(collection)
}

func createApiTestCasesCollection(app core.App) error {
	collection := core.NewBaseCollection("api_test_cases")
	authRule := "@request.auth.id != \"\""

	collection.ListRule = &authRule
	collection.ViewRule = &authRule
	collection.CreateRule = &authRule
	collection.UpdateRule = &authRule
	collection.DeleteRule = &authRule

	collectionsCollection, err := app.FindCollectionByNameOrId("api_test_collections")
	if err != nil {
		return err
	}

	minZero := 0.0
	minOne := 1.0
	maxStatus := 599.0
	maxMinutes := 1440.0
	maxThreshold := 100.0
	maxTimeout := 120000.0

	collection.Fields.Add(&core.RelationField{
		Name:          "collection",
		CollectionId:  collectionsCollection.Id,
		Required:      true,
		MaxSelect:     1,
		CascadeDelete: true,
	})
	collection.Fields.Add(&core.TextField{Name: "name", Required: true})
	collection.Fields.Add(&core.SelectField{
		Name:      "method",
		Required:  true,
		MaxSelect: 1,
		Values:    []string{"GET", "POST", "PUT", "DELETE", "PATCH", "HEAD"},
	})
	collection.Fields.Add(&core.TextField{Name: "url", Required: true})
	collection.Fields.Add(&core.TextField{Name: "description"})
	collection.Fields.Add(&core.JSONField{Name: "headers"})
	collection.Fields.Add(&core.JSONField{Name: "params"})
	collection.Fields.Add(&core.SelectField{
		Name:      "body_type",
		Required:  true,
		MaxSelect: 1,
		Values:    []string{"json", "text", "form"},
	})
	collection.Fields.Add(&core.TextField{Name: "body"})
	collection.Fields.Add(&core.NumberField{Name: "expected_status", OnlyInt: true, Min: &minZero, Max: &maxStatus})
	collection.Fields.Add(&core.NumberField{Name: "timeout_ms", OnlyInt: true, Min: &minZero, Max: &maxTimeout})
	collection.Fields.Add(&core.BoolField{Name: "schedule_enabled"})
	collection.Fields.Add(&core.NumberField{Name: "schedule_minutes", OnlyInt: true, Min: &minOne, Max: &maxMinutes})
	collection.Fields.Add(&core.NumberField{Name: "sort_order", OnlyInt: true, Min: &minZero})
	collection.Fields.Add(&core.JSONField{Name: "tags"})
	collection.Fields.Add(&core.BoolField{Name: "alert_enabled"})
	collection.Fields.Add(&core.NumberField{Name: "alert_threshold", OnlyInt: true, Min: &minOne, Max: &maxThreshold})
	collection.Fields.Add(&core.NumberField{Name: "consecutive_failures", OnlyInt: true, Min: &minZero})
	collection.Fields.Add(&core.BoolField{Name: "alert_triggered"})
	collection.Fields.Add(&core.NumberField{Name: "last_status", OnlyInt: true, Min: &minZero, Max: &maxStatus})
	collection.Fields.Add(&core.NumberField{Name: "last_duration_ms", OnlyInt: true, Min: &minZero})
	collection.Fields.Add(&core.DateField{Name: "last_run_at"})
	collection.Fields.Add(&core.BoolField{Name: "last_success"})
	collection.Fields.Add(&core.TextField{Name: "last_error"})
	collection.Fields.Add(&core.TextField{Name: "last_response_snippet"})
	collection.Fields.Add(&core.AutodateField{Name: "created", OnCreate: true})
	collection.Fields.Add(&core.AutodateField{Name: "updated", OnCreate: true, OnUpdate: true})

	collection.AddIndex("idx_api_test_cases_collection_order", false, "collection,sort_order,created", "")
	collection.AddIndex("idx_api_test_cases_collection_name", true, "collection,name", "")
	collection.AddIndex("idx_api_test_cases_schedule", false, "schedule_enabled,schedule_minutes", "")

	return app.Save(collection)
}

func createApiTestScheduleConfigCollection(app core.App) error {
	collection := core.NewBaseCollection("api_test_schedule_config")
	authRule := "@request.auth.id != \"\""

	collection.ListRule = &authRule
	collection.ViewRule = &authRule
	collection.CreateRule = &authRule
	collection.UpdateRule = &authRule
	collection.DeleteRule = &authRule

	minOne := 1.0
	maxMinutes := 1440.0
	maxRetention := 365.0

	collection.Fields.Add(&core.BoolField{Name: "enabled"})
	collection.Fields.Add(&core.NumberField{Name: "interval_minutes", OnlyInt: true, Min: &minOne, Max: &maxMinutes})
	collection.Fields.Add(&core.DateField{Name: "last_run_at"})
	collection.Fields.Add(&core.DateField{Name: "next_run_at"})
	collection.Fields.Add(&core.TextField{Name: "last_error"})
	collection.Fields.Add(&core.BoolField{Name: "alert_enabled"})
	collection.Fields.Add(&core.BoolField{Name: "alert_on_recover"})
	collection.Fields.Add(&core.NumberField{Name: "history_retention_days", OnlyInt: true, Min: &minOne, Max: &maxRetention})
	collection.Fields.Add(&core.AutodateField{Name: "created", OnCreate: true})
	collection.Fields.Add(&core.AutodateField{Name: "updated", OnCreate: true, OnUpdate: true})

	return app.Save(collection)
}

func createApiTestRunsCollection(app core.App) error {
	collection := core.NewBaseCollection("api_test_runs")
	authRule := "@request.auth.id != \"\""

	collection.ListRule = &authRule
	collection.ViewRule = &authRule
	collection.CreateRule = nil
	collection.UpdateRule = nil
	collection.DeleteRule = nil

	collectionsCollection, err := app.FindCollectionByNameOrId("api_test_collections")
	if err != nil {
		return err
	}
	casesCollection, err := app.FindCollectionByNameOrId("api_test_cases")
	if err != nil {
		return err
	}

	minZero := 0.0
	maxStatus := 599.0

	collection.Fields.Add(&core.RelationField{
		Name:          "collection",
		CollectionId:  collectionsCollection.Id,
		Required:      true,
		MaxSelect:     1,
		CascadeDelete: true,
	})
	collection.Fields.Add(&core.RelationField{
		Name:          "case",
		CollectionId:  casesCollection.Id,
		Required:      true,
		MaxSelect:     1,
		CascadeDelete: true,
	})
	collection.Fields.Add(&core.NumberField{Name: "status", OnlyInt: true, Min: &minZero, Max: &maxStatus})
	collection.Fields.Add(&core.NumberField{Name: "duration_ms", OnlyInt: true, Min: &minZero})
	collection.Fields.Add(&core.BoolField{Name: "success"})
	collection.Fields.Add(&core.TextField{Name: "error"})
	collection.Fields.Add(&core.TextField{Name: "response_snippet"})
	collection.Fields.Add(&core.SelectField{
		Name:      "source",
		Required:  true,
		MaxSelect: 1,
		Values:    []string{"manual", "schedule"},
	})
	collection.Fields.Add(&core.AutodateField{Name: "created", OnCreate: true})

	collection.AddIndex("idx_api_test_runs_case_created", false, "case,created", "")
	collection.AddIndex("idx_api_test_runs_collection_created", false, "collection,created", "")

	return app.Save(collection)
}
