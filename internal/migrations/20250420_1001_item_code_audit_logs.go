// Item Code audit logs collection.
package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

const itemCodeAuditLogsCollection = "item_code_audit_logs"

func init() {
	m.Register(func(app core.App) error {
		return createItemCodeAuditLogsCollection(app)
	}, func(app core.App) error {
		return deleteCollection(app, itemCodeAuditLogsCollection)
	})
}

func createItemCodeAuditLogsCollection(app core.App) error {
	collection := core.NewBaseCollection(itemCodeAuditLogsCollection)

	usersCollection, err := app.FindCollectionByNameOrId("users")
	if err != nil {
		return err
	}

	adminRule := "@request.auth.id != '' && @request.auth.role = 'admin'"

	collection.ListRule = &adminRule
	collection.ViewRule = &adminRule
	collection.CreateRule = &adminRule
	collection.UpdateRule = nil
	collection.DeleteRule = nil

	collection.Fields.Add(&core.RelationField{
		Name:         "user",
		CollectionId: usersCollection.Id,
		MaxSelect:    1,
	})
	collection.Fields.Add(&core.TextField{
		Name:     "action",
		Required: true,
	})
	collection.Fields.Add(&core.TextField{
		Name: "target_ids",
	})
	collection.Fields.Add(&core.TextField{
		Name: "filter",
	})
	collection.Fields.Add(&core.SelectField{
		Name:      "status",
		Required:  true,
		MaxSelect: 1,
		Values:    []string{"success", "failed"},
	})
	collection.Fields.Add(&core.TextField{
		Name: "detail",
	})
	collection.Fields.Add(&core.TextField{
		Name: "ip_address",
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

	collection.AddIndex("idx_item_code_audit_logs_action", false, "action", "")
	collection.AddIndex("idx_item_code_audit_logs_status", false, "status", "")
	collection.AddIndex("idx_item_code_audit_logs_user", false, "user", "")
	collection.AddIndex("idx_item_code_audit_logs_created", false, "created", "")

	return app.Save(collection)
}
