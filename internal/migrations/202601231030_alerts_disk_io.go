// 告警集合的 name 字段新增 DiskIO 选项。
package migrations

import (
	"fmt"

	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

func init() {
	m.Register(func(app core.App) error {
		return updateAlertNameValues(app, true)
	}, func(app core.App) error {
		return updateAlertNameValues(app, false)
	})
}

func updateAlertNameValues(app core.App, add bool) error {
	collection, err := app.FindCollectionByNameOrId("alerts")
	if err != nil {
		return err
	}

	field := collection.Fields.GetByName("name")
	selectField, ok := field.(*core.SelectField)
	if !ok {
		return fmt.Errorf("alerts.name field is not a select field")
	}

	values := selectField.Values
	if add {
		if !stringSliceContains(values, "DiskIO") {
			selectField.Values = append(values, "DiskIO")
		}
		return app.Save(collection)
	}

	filtered := values[:0]
	for _, value := range values {
		if value != "DiskIO" {
			filtered = append(filtered, value)
		}
	}
	selectField.Values = filtered
	return app.Save(collection)
}

func stringSliceContains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
