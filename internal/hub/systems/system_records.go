package systems

import (
	"fmt"
	"strings"

	"github.com/pocketbase/dbx"
	"github.com/pocketbase/pocketbase/core"
)

func deleteStaleSystemRecords(app core.App, table, systemID string, ids []string) error {
	params := dbx.Params{
		"system": systemID,
	}
	if len(ids) == 0 {
		_, err := app.DB().NewQuery(fmt.Sprintf("DELETE FROM %s WHERE system = {:system}", table)).Bind(params).Execute()
		return err
	}

	placeholders := make([]string, 0, len(ids))
	for i, id := range ids {
		key := fmt.Sprintf("id%d", i)
		params[key] = id
		placeholders = append(placeholders, "{:"+key+"}")
	}

	query := fmt.Sprintf("DELETE FROM %s WHERE system = {:system} AND id NOT IN (%s)", table, strings.Join(placeholders, ","))
	_, err := app.DB().NewQuery(query).Bind(params).Execute()
	return err
}
