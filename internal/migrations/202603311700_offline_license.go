// offline license device collection for activation requests and current licenses.
package migrations

import (
	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

const (
	offlineLicenseActivationsCollection = "offline_license_activations"
)

func init() {
	m.Register(func(app core.App) error {
		return createOfflineLicenseActivationsCollection(app)
	}, func(app core.App) error {
		return deleteCollection(app, offlineLicenseActivationsCollection)
	})
}

func createOfflineLicenseActivationsCollection(app core.App) error {
	collection := core.NewBaseCollection(offlineLicenseActivationsCollection)
	adminRule := "@request.auth.id != \"\" && @request.auth.role = \"admin\""

	collection.ListRule = &adminRule
	collection.ViewRule = &adminRule
	collection.CreateRule = &adminRule
	collection.UpdateRule = &adminRule
	collection.DeleteRule = &adminRule

	systemsCollection, err := app.FindCollectionByNameOrId("systems")
	if err != nil {
		return err
	}

	collection.Fields.Add(&core.TextField{Name: "request_id", Required: true})
	collection.Fields.Add(&core.RelationField{
		Name:         "system",
		CollectionId: systemsCollection.Id,
		MaxSelect:    1,
	})
	collection.Fields.Add(&core.TextField{Name: "customer"})
	collection.Fields.Add(&core.TextField{Name: "tenant"})
	collection.Fields.Add(&core.TextField{Name: "project_name"})
	collection.Fields.Add(&core.TextField{Name: "site_name"})
	collection.Fields.Add(&core.TextField{Name: "remarks"})
	collection.Fields.Add(&core.TextField{Name: "fingerprint", Required: true})
	collection.Fields.Add(&core.TextField{Name: "hostname"})
	collection.Fields.Add(&core.JSONField{Name: "factors_json", Required: true})
	collection.Fields.Add(&core.TextField{Name: "device_public_key_pem", Required: true})
	collection.Fields.Add(&core.JSONField{Name: "activation_payload", Required: true})
	collection.Fields.Add(&core.SelectField{
		Name:      "status",
		Required:  true,
		MaxSelect: 1,
		Values:    []string{"imported", "active", "disabled"},
	})
	collection.Fields.Add(&core.DateField{Name: "last_issued_at"})
	collection.Fields.Add(&core.TextField{Name: "current_license_id"})
	collection.Fields.Add(&core.TextField{Name: "current_export_name"})
	collection.Fields.Add(&core.DateField{Name: "current_not_before"})
	collection.Fields.Add(&core.DateField{Name: "current_not_after"})
	collection.Fields.Add(&core.JSONField{Name: "current_models_json"})
	collection.Fields.Add(&core.JSONField{Name: "current_license_payload"})
	collection.Fields.Add(&core.TextField{Name: "current_license_signature"})
	collection.Fields.Add(&core.AutodateField{Name: "created", OnCreate: true})
	collection.Fields.Add(&core.AutodateField{Name: "updated", OnCreate: true, OnUpdate: true})

	collection.AddIndex("idx_offline_license_activations_request_id", true, "request_id", "")
	collection.AddIndex("idx_offline_license_activations_status_created", false, "status,created", "")
	collection.AddIndex("idx_offline_license_activations_customer_updated", false, "customer,updated", "")
	collection.AddIndex("idx_offline_license_activations_current_license_id", false, "current_license_id", "")

	return app.Save(collection)
}
