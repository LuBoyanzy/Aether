// 迁移移除 api_test_cases.alert_enabled，统一使用全局告警开关。
package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

func init() {
	m.Register(func(app core.App) error {
		collection, err := app.FindCollectionByNameOrId("api_test_cases")
		if err != nil {
			return err
		}

		collection.Fields.RemoveByName("alert_enabled")

		return app.Save(collection)
	}, func(app core.App) error {
		collection, err := app.FindCollectionByNameOrId("api_test_cases")
		if err != nil {
			return err
		}

		collection.Fields.Add(&core.BoolField{Name: "alert_enabled"})

		return app.Save(collection)
	})
}
