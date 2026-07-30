package db

import (
	"context"
	"strings"
)

func listDatabasesMySQL(ctx context.Context, sess *Session) ([]string, error) {
	rows, err := sess.DB.QueryContext(ctx, "SHOW DATABASES")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		out = append(out, name)
	}
	return out, rows.Err()
}

func useDatabaseMySQL(ctx context.Context, sess *Session, database string) error {
	q, err := quoteIdent(DriverMySQL, database)
	if err != nil {
		return err
	}
	_, err = sess.DB.ExecContext(ctx, "USE "+q)
	return err
}

func listTablesMySQL(ctx context.Context, sess *Session) ([]TableInfo, error) {
	rows, err := sess.DB.QueryContext(ctx, "SHOW FULL TABLES")
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
		t := "table"
		if strings.EqualFold(typ, "VIEW") {
			t = "view"
		}
		out = append(out, TableInfo{Name: name, Type: t})
	}
	return out, rows.Err()
}

func listColumnsMySQL(ctx context.Context, sess *Session, table string) ([]Column, error) {
	tq, err := quoteIdent(DriverMySQL, table)
	if err != nil {
		return nil, err
	}
	rows, err := sess.DB.QueryContext(ctx, "SHOW FULL COLUMNS FROM "+tq)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Column
	for rows.Next() {
		var field, typ, null, key, extra, privileges, comment string
		var def *string
		var collation *string
		if err := rows.Scan(&field, &typ, &collation, &null, &key, &def, &extra, &privileges, &comment); err != nil {
			return nil, err
		}
		out = append(out, Column{
			Name:     field,
			Type:     typ,
			Nullable: strings.EqualFold(null, "YES"),
			Primary:  key == "PRI",
			Default:  def,
		})
	}
	return out, rows.Err()
}
