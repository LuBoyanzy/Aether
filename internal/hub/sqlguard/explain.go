package sqlguard

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
)

// explainPlan mirrors the JSON output of EXPLAIN (FORMAT JSON).
type explainPlan struct {
	Plan explainNode `json:"Plan"`
}

// explainNode mirrors a single node in the PostgreSQL plan tree.
type explainNode struct {
	NodeType     string        `json:"Node Type"`
	RelationName string        `json:"Relation Name"`
	Plans        []explainNode `json:"Plans,omitempty"`
	SubplanName  string        `json:"Subplan Name,omitempty"`
}

// ValidateViaExplain runs EXPLAIN (FORMAT JSON) in a read-only transaction
// and verifies that the query only touches the allowed table and contains
// no forbidden operations (subqueries, functions, etc.).
func ValidateViaExplain(ctx context.Context, db *sql.DB, query string) error {
	tx, err := db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return fmt.Errorf("begin read-only tx failed: %w", err)
	}
	defer tx.Rollback()

	var jsonStr string
	if err := tx.QueryRowContext(ctx, "EXPLAIN (FORMAT JSON) "+query).Scan(&jsonStr); err != nil {
		return fmt.Errorf("invalid SQL: %w", err)
	}

	var plans []explainPlan
	if err := json.Unmarshal([]byte(jsonStr), &plans); err != nil {
		return fmt.Errorf("explain parse failed: %w", err)
	}
	if len(plans) == 0 {
		return fmt.Errorf("empty explain plan")
	}

	return validateNode(plans[0].Plan)
}

func validateNode(node explainNode) error {
	// Reject specific scan / execution types that indicate subqueries or functions.
	switch node.NodeType {
	case "Function Scan", "Materialize",
		"WorkTable Scan", "Named Tuplestore Scan":
		return fmt.Errorf("forbidden operation: %s", node.NodeType)
	}

	// Reject any subplan.
	if node.SubplanName != "" {
		return fmt.Errorf("subplan is not allowed")
	}

	// Ensure we only touch the allowed table.
	// CTE scans reference temporary CTE names, not real tables — skip check.
	if node.NodeType != "CTE Scan" && node.RelationName != "" && node.RelationName != allowedTable {
		return fmt.Errorf("forbidden table access: %s", node.RelationName)
	}

	// Recurse into child plans.
	for _, child := range node.Plans {
		if err := validateNode(child); err != nil {
			return err
		}
	}
	return nil
}
