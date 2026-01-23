// 通知设置集合迁移：新增全局通知语言配置。
package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

func init() {
	m.Register(func(app core.App) error {
		collection := core.NewBaseCollection("notification_settings")
		authRule := "@request.auth.id != \"\""

		collection.ListRule = &authRule
		collection.ViewRule = &authRule
		collection.CreateRule = &authRule
		collection.UpdateRule = &authRule
		collection.DeleteRule = &authRule

		collection.Fields.Add(&core.SelectField{
			Name:      "language",
			Required:  true,
			MaxSelect: 1,
			Values:    []string{"zh-CN", "en"},
		})
		collection.Fields.Add(&core.AutodateField{Name: "created", OnCreate: true})
		collection.Fields.Add(&core.AutodateField{Name: "updated", OnCreate: true, OnUpdate: true})

		if err := app.Save(collection); err != nil {
			return err
		}

		record := core.NewRecord(collection)
		record.Set("language", "zh-CN")
		return app.Save(record)
	}, func(app core.App) error {
		return deleteCollection(app, "notification_settings")
	})
}
