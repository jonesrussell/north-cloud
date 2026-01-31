package database

import "database/sql"

// execRequireRows validates that an ExecContext result affected at least one row.
// Returns err if non-nil, or notFoundErr if rowsAffected is 0.
func execRequireRows(result sql.Result, err, notFoundErr error) error {
	if err != nil {
		return err
	}
	n, affectedErr := result.RowsAffected()
	if affectedErr != nil {
		return affectedErr
	}
	if n == 0 {
		return notFoundErr
	}
	return nil
}
