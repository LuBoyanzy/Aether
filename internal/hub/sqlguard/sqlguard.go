// Package sqlguard provides SQL validation and rewriting for query-delete operations.
package sqlguard

import (
	"fmt"
	"regexp"
	"strings"
)

const allowedTable = "product_info"

var (
	// deletePrefix matches: DELETE FROM product_info WHERE <rest>
	deletePrefix = regexp.MustCompile(`(?i)^\s*DELETE\s+FROM\s+` + allowedTable + `\s+WHERE\s+(.+)$`)

	// selectPrefix matches: SELECT ... FROM product_info WHERE <rest>
	selectPrefix = regexp.MustCompile(`(?i)^\s*SELECT\s+.+FROM\s+` + allowedTable + `\s+WHERE\s+(.+)$`)

	// subqueryPattern detects (SELECT ...)
	subqueryPattern = regexp.MustCompile(`(?i)\(\s*SELECT\s+`)

	// tenantIDPattern prevents users from manipulating tenant isolation
	tenantIDPattern = regexp.MustCompile(`(?i)\btenant_id\b`)
)

// forbiddenKeywords are dangerous SQL keywords that are never allowed.
var forbiddenKeywords = []string{
	";", "UNION", "INSERT", "UPDATE", "DROP", "TRUNCATE", "ALTER",
	"CREATE", "GRANT", "EXECUTE", "COPY", "LOAD", "VACUUM", "ANALYZE",
}

// ValidateSQL performs L1 (prefix whitelist) and L2 (keyword blacklist) validation.
// It returns the extracted WHERE clause, whether the statement is a DELETE, and any error.
func ValidateSQL(sql string) (whereClause string, isDelete bool, err error) {
	sqlTrim := strings.TrimSpace(sql)
	sqlUpper := strings.ToUpper(sqlTrim)

	// L2: forbidden keywords
	for _, kw := range forbiddenKeywords {
		if strings.Contains(sqlUpper, kw) {
			return "", false, fmt.Errorf("forbidden keyword: %s", kw)
		}
	}

	// L2: subqueries
	if subqueryPattern.MatchString(sqlUpper) {
		return "", false, fmt.Errorf("subquery is not allowed")
	}

	// L2: tenant_id is reserved
	if tenantIDPattern.MatchString(sqlUpper) {
		return "", false, fmt.Errorf("tenant_id is reserved and cannot be used in user SQL")
	}

	// L1: match DELETE
	if m := deletePrefix.FindStringSubmatch(sqlTrim); m != nil {
		return m[1], true, nil
	}

	// L1: match SELECT
	if m := selectPrefix.FindStringSubmatch(sqlTrim); m != nil {
		return m[1], false, nil
	}

	return "", false, fmt.Errorf("only SELECT or DELETE FROM product_info WHERE ... is allowed")
}
