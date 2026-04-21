package main

import (
	"fmt"
	"github.com/xwb1989/sqlparser"
)

func hasSubquery(node sqlparser.SQLNode) bool {
	hasSub := false
	sqlparser.Walk(func(node sqlparser.SQLNode) (kontinue bool, err error) {
		if _, ok := node.(*sqlparser.Subquery); ok {
			hasSub = true
			return false, nil
		}
		return true, nil
	}, node)
	return hasSub
}

func main() {
	sql := "DELETE FROM product_info WHERE item_code = (SELECT item_code FROM other_table)"
	stmt, err := sqlparser.Parse(sql)
	if err != nil {
		fmt.Println("Parse error:", err)
		return
	}
	if del, ok := stmt.(*sqlparser.Delete); ok {
		fmt.Printf("Has subquery: %v\n", hasSubquery(del.Where))
	}
}
