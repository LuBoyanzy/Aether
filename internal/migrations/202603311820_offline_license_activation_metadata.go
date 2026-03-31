// Migration consolidates offline license data into a single device registry collection.
package migrations

import (
	"fmt"

	"github.com/pocketbase/pocketbase/core"
	m "github.com/pocketbase/pocketbase/migrations"
)

func init() {
	m.Register(func(app core.App) error {
		if err := upgradeOfflineLicenseActivationsCollection(app); err != nil {
			return err
		}
		if err := remapOfflineLicenseActivationStatuses(app, map[string]string{
			"issued":  "active",
			"revoked": "disabled",
		}); err != nil {
			return err
		}
		return deleteCollectionIfExists(app, "offline_license_artifacts")
	}, func(app core.App) error {
		if err := remapOfflineLicenseActivationStatuses(app, map[string]string{
			"active":   "issued",
			"disabled": "revoked",
		}); err != nil {
			return err
		}
		return downgradeOfflineLicenseActivationsCollection(app)
	})
}

func upgradeOfflineLicenseActivationsCollection(app core.App) error {
	collection, err := app.FindCollectionByNameOrId(offlineLicenseActivationsCollection)
	if err != nil {
		return err
	}

	addOfflineLicenseTextField(collection, "customer", true)
	addOfflineLicenseTextField(collection, "tenant", false)
	addOfflineLicenseTextField(collection, "project_name", false)
	addOfflineLicenseTextField(collection, "site_name", false)
	addOfflineLicenseTextField(collection, "remarks", false)
	addOfflineLicenseTextField(collection, "current_license_id", false)
	addOfflineLicenseTextField(collection, "current_export_name", false)
	addOfflineLicenseTextField(collection, "current_license_signature", false)
	addOfflineLicenseDateField(collection, "current_not_before")
	addOfflineLicenseDateField(collection, "current_not_after")
	addOfflineLicenseJSONField(collection, "current_models_json")
	addOfflineLicenseJSONField(collection, "current_license_payload")

	if field := collection.Fields.GetByName("status"); field != nil {
		selectField, ok := field.(*core.SelectField)
		if !ok {
			return fmt.Errorf("offline license activation status field is not a select field")
		}
		selectField.Values = []string{"imported", "active", "disabled"}
	}

	collection.AddIndex("idx_offline_license_activations_customer_updated", false, "customer,updated", "")
	collection.AddIndex("idx_offline_license_activations_current_license_id", false, "current_license_id", "")
	return app.Save(collection)
}

func downgradeOfflineLicenseActivationsCollection(app core.App) error {
	collection, err := app.FindCollectionByNameOrId(offlineLicenseActivationsCollection)
	if err != nil {
		return err
	}

	collection.Fields.RemoveByName("customer")
	collection.Fields.RemoveByName("tenant")
	collection.Fields.RemoveByName("project_name")
	collection.Fields.RemoveByName("site_name")
	collection.Fields.RemoveByName("remarks")
	collection.Fields.RemoveByName("current_license_id")
	collection.Fields.RemoveByName("current_export_name")
	collection.Fields.RemoveByName("current_license_signature")
	collection.Fields.RemoveByName("current_not_before")
	collection.Fields.RemoveByName("current_not_after")
	collection.Fields.RemoveByName("current_models_json")
	collection.Fields.RemoveByName("current_license_payload")

	if field := collection.Fields.GetByName("status"); field != nil {
		selectField, ok := field.(*core.SelectField)
		if !ok {
			return fmt.Errorf("offline license activation status field is not a select field")
		}
		selectField.Values = []string{"imported", "issued", "revoked"}
	}

	return app.Save(collection)
}

func addOfflineLicenseTextField(collection *core.Collection, name string, required bool) {
	if collection.Fields.GetByName(name) != nil {
		return
	}
	collection.Fields.Add(&core.TextField{
		Name:     name,
		Required: required,
	})
}

func addOfflineLicenseDateField(collection *core.Collection, name string) {
	if collection.Fields.GetByName(name) != nil {
		return
	}
	collection.Fields.Add(&core.DateField{Name: name})
}

func addOfflineLicenseJSONField(collection *core.Collection, name string) {
	if collection.Fields.GetByName(name) != nil {
		return
	}
	collection.Fields.Add(&core.JSONField{Name: name})
}

func remapOfflineLicenseActivationStatuses(app core.App, mapping map[string]string) error {
	records, err := app.FindAllRecords(offlineLicenseActivationsCollection)
	if err != nil {
		return err
	}

	for _, record := range records {
		current := record.GetString("status")
		next, ok := mapping[current]
		if !ok || next == current {
			continue
		}
		record.Set("status", next)
		if err := app.SaveNoValidate(record); err != nil {
			return err
		}
	}

	return nil
}

func deleteCollectionIfExists(app core.App, name string) error {
	collection, err := app.FindCollectionByNameOrId(name)
	if err != nil {
		return nil
	}
	return app.Delete(collection)
}
