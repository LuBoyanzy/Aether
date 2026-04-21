package sqlguard

import "fmt"

// BuildPreviewSelect assembles a safe SELECT statement from the user's WHERE clause.
func BuildPreviewSelect(whereClause string) string {
	return fmt.Sprintf(`
		SELECT
			item_code,
			COALESCE(product_name, '') AS product_name,
			COALESCE(category_name, '') AS category_name,
			COALESCE(description, '') AS description,
			update_time,
			CASE
				WHEN COALESCE(is_deleted, false) = true THEN 'obsolete'
				WHEN has_3d_model = true AND has_2d_image = true THEN 'active'
				ELSE 'inactive'
			END AS status
		FROM product_info
		WHERE %s
		  AND tenant_id = current_setting('app.current_tenant')
		  AND COALESCE(is_deleted, false) = false
		LIMIT 100
	`, whereClause)
}

// BuildDelete assembles a safe DELETE statement from the user's WHERE clause.
// Uses a CTE to enforce the 1000-row limit because PostgreSQL does not support
// LIMIT on DELETE directly.
func BuildDelete(whereClause string) string {
	return fmt.Sprintf(`
		WITH to_delete AS (
			SELECT item_code FROM product_info
			WHERE %s
			  AND tenant_id = current_setting('app.current_tenant')
			  AND COALESCE(is_deleted, false) = false
			LIMIT 1000
		)
		DELETE FROM product_info
		WHERE item_code IN (SELECT item_code FROM to_delete)
		RETURNING item_code
	`, whereClause)
}
