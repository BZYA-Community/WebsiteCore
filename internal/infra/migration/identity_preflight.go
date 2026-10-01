package migration

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

// CheckIdentityUpgrade is read-only and runs before the migration driver, which
// can create its version table or mark a migration dirty. Include deleted users.
func CheckIdentityUpgrade(ctx context.Context, db *sql.DB, prefix string) error {
	table := prefix + "user"
	var exists bool
	if err := db.QueryRowContext(ctx, "SELECT to_regclass($1) IS NOT NULL", table).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		return nil
	}
	var upgraded bool
	if err := db.QueryRowContext(ctx, `SELECT EXISTS (
        SELECT 1 FROM pg_attribute WHERE attrelid = to_regclass($1)
        AND attname = 'member_identity' AND NOT attisdropped)`, table).Scan(&upgraded); err != nil {
		return err
	}
	if upgraded {
		return nil
	}
	quoted := `"` + strings.ReplaceAll(table, `"`, `""`) + `"`
	if err := db.QueryRowContext(ctx, "SELECT EXISTS (SELECT 1 FROM "+quoted+")").Scan(&exists); err != nil {
		return err
	}
	if exists {
		return fmt.Errorf("identity upgrade requires an empty user table, including deleted accounts; no data migration is supported")
	}
	return nil
}
