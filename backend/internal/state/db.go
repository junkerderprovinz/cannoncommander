package state

import "database/sql"

// MainDB returns the main database for the /api/v1 packages, which keep
// their own tables there through named migrations. It is nil before Open.
func MainDB() *sql.DB {
	if sqlDb == nil {
		return nil
	}
	return sqlDb.DB()
}
