package sqlguard

import (
	"strings"
	"testing"
)

func TestBuildPreviewSelectUsesMissingOKTenantSetting(t *testing.T) {
	query := BuildPreviewSelect("item_code = 'A001'")

	if !strings.Contains(query, "current_setting('app.current_tenant', true)") {
		t.Fatalf("preview query should tolerate EXPLAIN before tenant is set:\n%s", query)
	}
}

func TestBuildDeleteScopesOuterDeleteByTenant(t *testing.T) {
	query := BuildDelete("item_code = 'A001'")
	outerDeleteStart := strings.Index(query, "DELETE FROM product_info")
	if outerDeleteStart < 0 {
		t.Fatalf("delete query should contain outer DELETE:\n%s", query)
	}
	outerDelete := query[outerDeleteStart:]

	if !strings.Contains(outerDelete, "tenant_id = current_setting('app.current_tenant', true)") {
		t.Fatalf("outer delete should be tenant scoped:\n%s", query)
	}
}

func TestBuildDeleteCandidatesUsesTenantAndLimit(t *testing.T) {
	query := BuildDeleteCandidates("category_name = '标准件'")

	if !strings.Contains(query, "tenant_id = current_setting('app.current_tenant', true)") {
		t.Fatalf("candidate query should be tenant scoped:\n%s", query)
	}
	if !strings.Contains(query, "LIMIT 1000") {
		t.Fatalf("candidate query should cap destructive operations:\n%s", query)
	}
}
