// Item Code management collection.
package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

const itemCodesCollection = "item_codes"

func init() {
	m.Register(func(app core.App) error {
		return createItemCodesCollection(app)
	}, func(app core.App) error {
		return deleteCollection(app, itemCodesCollection)
	})
}

func createItemCodesCollection(app core.App) error {
	collection := core.NewBaseCollection(itemCodesCollection)

	readRule := "@request.auth.id != ''"
	writeRule := readRule + " && @request.auth.role != 'readonly'"

	collection.ListRule = &readRule
	collection.ViewRule = &readRule
	collection.CreateRule = &writeRule
	collection.UpdateRule = &writeRule
	collection.DeleteRule = &writeRule

	collection.Fields.Add(&core.TextField{
		Name:     "code",
		Required: true,
	})
	collection.Fields.Add(&core.TextField{
		Name:     "name",
		Required: true,
	})
	collection.Fields.Add(&core.TextField{
		Name: "category",
	})
	collection.Fields.Add(&core.SelectField{
		Name:      "status",
		Required:  true,
		MaxSelect: 1,
		Values:    []string{"active", "inactive", "obsolete"},
	})
	collection.Fields.Add(&core.TextField{
		Name: "description",
	})
	collection.Fields.Add(&core.AutodateField{
		Name:     "created",
		OnCreate: true,
	})
	collection.Fields.Add(&core.AutodateField{
		Name:     "updated",
		OnCreate: true,
		OnUpdate: true,
	})

	collection.AddIndex("idx_item_codes_code", true, "code", "")
	collection.AddIndex("idx_item_codes_status", false, "status", "")
	collection.AddIndex("idx_item_codes_category", false, "category", "")

	return app.Save(collection)
}
