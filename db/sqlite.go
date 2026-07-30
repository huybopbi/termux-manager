package db

import (
	"context"
	"fmt"
)

func listTablesSQLite(ctx context.Context, sess *Session) ([]TableInfo, error) {
	rows, err := sess.DB.QueryContext(ctx, `
		SELECT name, type FROM sqlite_master
		WHERE type IN ('table','view') AND name NOT LIKE 'sqlite_%'
		ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []TableInfo
	for rows.Next() {
		var name, typ string
		if err := rows.Scan(&name, &typ); err != nil {
			return nil, err
		}
		out = append(out, TableInfo{Name: name, Type: typ})
	}
	return out, rows.Err()
}

func listColumnsSQLite(ctx context.Context, sess *Session, table string) ([]Column, error) {
	// PRAGMA table_info does not accept bound params for table name in all builds;
	// validate via quoteIdent then embed.
	if _, err := quoteIdent(DriverSQLite, table); err != nil {
		return nil, err
	}
	// Use bound form via pragma_table_info when available
	rows, err := sess.DB.QueryContext(ctx, `SELECT name, type, "notnull", dflt_value, pk FROM pragma_table_info(?)`, table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Column
	for rows.Next() {
		var name, typ string
		var notnull, pk int
		var def *string
		if err := rows.Scan(&name, &typ, &notnull, &def, &pk); err != nil {
			return nil, err
		}
		out = append(out, Column{
			Name:     name,
			Type:     typ,
			Nullable: notnull == 0,
			Primary:  pk > 0,
			Default:  def,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(out) == 0 {
		// distinguish missing table
		var n int
		err := sess.DB.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM sqlite_master WHERE type IN ('table','view') AND name=?`, table).Scan(&n)
		if err != nil {
			return nil, err
		}
		if n == 0 {
			return nil, fmt.Errorf("table not found: %s", table)
		}
	}
	return out, nil
}
