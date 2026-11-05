package fsindex

import (
	"context"
	"path"
	"strings"

	"github.com/junkerderprovinz/cannoncommander/backend/internal/ccfs"
	"github.com/junkerderprovinz/cannoncommander/backend/internal/database/sqlitebusy"
	"github.com/junkerderprovinz/cannoncommander/backend/pkg/indexing"
)

// Parts returns the API paths of the part files the index knows in src, of the
// copy jobs and uploads as well as of WebDAV.
func Parts(ctx context.Context, src *ccfs.Source) ([]string, error) {
	db := indexing.GetIndexDB()
	if db == nil {
		return nil, nil
	}
	rows, err := db.DB().QueryContext(ctx, `SELECT path FROM index_items
		WHERE source = ? AND is_dir = 0 AND name LIKE ?`, src.Name, ".%"+ccfs.PartSuffix)
	if err != nil {
		return nil, sqlitebusy.Wrap(err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			return nil, err
		}
		out = append(out, path.Join(src.Mount, strings.TrimSuffix(key, "/")))
	}
	return out, rows.Err()
}
