package db

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

func withTimeout() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), time.Duration(QueryTimeout)*time.Second)
}

func ListDatabases(sess *Session) ([]string, error) {
	ctx, cancel := withTimeout()
	defer cancel()
	switch sess.Driver {
	case DriverMySQL:
		return listDatabasesMySQL(ctx, sess)
	case DriverSQLite:
		if sess.Database != "" {
			return []string{sess.Database}, nil
		}
		return []string{sess.Path}, nil
	default:
		return nil, fmt.Errorf("unsupported driver")
	}
}

func UseDatabase(sess *Session, store *Store, database string) error {
	if sess.Driver != DriverMySQL {
		return fmt.Errorf("use database only supported for mysql")
	}
	ctx, cancel := withTimeout()
	defer cancel()
	if err := useDatabaseMySQL(ctx, sess, database); err != nil {
		return err
	}
	return store.UpdateDatabase(sess.ID, database)
}

func ListTables(sess *Session) ([]TableInfo, error) {
	ctx, cancel := withTimeout()
	defer cancel()
	switch sess.Driver {
	case DriverMySQL:
		return listTablesMySQL(ctx, sess)
	case DriverSQLite:
		return listTablesSQLite(ctx, sess)
	default:
		return nil, fmt.Errorf("unsupported driver")
	}
}

func ListColumns(sess *Session, table string) ([]Column, error) {
	ctx, cancel := withTimeout()
	defer cancel()
	switch sess.Driver {
	case DriverMySQL:
		return listColumnsMySQL(ctx, sess, table)
	case DriverSQLite:
		return listColumnsSQLite(ctx, sess, table)
	default:
		return nil, fmt.Errorf("unsupported driver")
	}
}

func ListRows(sess *Session, table string, limit, offset int) (*RowsResult, error) {
	limit = clampLimit(limit)
	if offset < 0 {
		offset = 0
	}
	tq, err := quoteIdent(sess.Driver, table)
	if err != nil {
		return nil, err
	}
	ctx, cancel := withTimeout()
	defer cancel()
	// fetch one extra to detect has_more
	q := fmt.Sprintf("SELECT * FROM %s LIMIT ? OFFSET ?", tq)
	rows, err := sess.DB.QueryContext(ctx, q, limit+1, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	cols, data, err := readRows(rows, limit+1)
	if err != nil {
		return nil, err
	}
	hasMore := len(data) > limit
	if hasMore {
		data = data[:limit]
	}
	return &RowsResult{
		Columns: cols,
		Rows:    data,
		Limit:   limit,
		Offset:  offset,
		HasMore: hasMore,
	}, nil
}

func InsertRow(sess *Session, table string, values map[string]interface{}) error {
	if len(values) == 0 {
		return fmt.Errorf("values required")
	}
	tq, err := quoteIdent(sess.Driver, table)
	if err != nil {
		return err
	}
	cols := make([]string, 0, len(values))
	placeholders := make([]string, 0, len(values))
	args := make([]interface{}, 0, len(values))
	for k, v := range values {
		cq, err := quoteIdent(sess.Driver, k)
		if err != nil {
			return err
		}
		cols = append(cols, cq)
		placeholders = append(placeholders, "?")
		args = append(args, normalizeWrite(v))
	}
	q := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)", tq, strings.Join(cols, ", "), strings.Join(placeholders, ", "))
	ctx, cancel := withTimeout()
	defer cancel()
	_, err = sess.DB.ExecContext(ctx, q, args...)
	return err
}

func UpdateRow(sess *Session, table string, pk, values map[string]interface{}) error {
	if len(pk) == 0 {
		return fmt.Errorf("primary key required for update")
	}
	if len(values) == 0 {
		return fmt.Errorf("values required")
	}
	tq, err := quoteIdent(sess.Driver, table)
	if err != nil {
		return err
	}
	sets := make([]string, 0, len(values))
	args := make([]interface{}, 0, len(values)+len(pk))
	for k, v := range values {
		cq, err := quoteIdent(sess.Driver, k)
		if err != nil {
			return err
		}
		sets = append(sets, cq+" = ?")
		args = append(args, normalizeWrite(v))
	}
	where, wargs, err := buildWhere(sess.Driver, pk)
	if err != nil {
		return err
	}
	args = append(args, wargs...)
	q := fmt.Sprintf("UPDATE %s SET %s WHERE %s", tq, strings.Join(sets, ", "), where)
	ctx, cancel := withTimeout()
	defer cancel()
	res, err := sess.DB.ExecContext(ctx, q, args...)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("no rows updated")
	}
	return nil
}

func DeleteRow(sess *Session, table string, pk map[string]interface{}) error {
	if len(pk) == 0 {
		return fmt.Errorf("primary key required for delete")
	}
	tq, err := quoteIdent(sess.Driver, table)
	if err != nil {
		return err
	}
	where, wargs, err := buildWhere(sess.Driver, pk)
	if err != nil {
		return err
	}
	q := fmt.Sprintf("DELETE FROM %s WHERE %s", tq, where)
	ctx, cancel := withTimeout()
	defer cancel()
	res, err := sess.DB.ExecContext(ctx, q, wargs...)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("no rows deleted")
	}
	return nil
}

func ExecQuery(sess *Session, sqlText string) (*QueryResult, error) {
	sqlText = strings.TrimSpace(sqlText)
	if sqlText == "" {
		return nil, fmt.Errorf("sql required")
	}
	ctx, cancel := withTimeout()
	defer cancel()
	if isSelectSQL(sqlText) {
		rows, err := sess.DB.QueryContext(ctx, sqlText)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		cols, data, err := readRows(rows, MaxRowLimit+1)
		if err != nil {
			return nil, err
		}
		if len(data) > MaxRowLimit {
			data = data[:MaxRowLimit]
		}
		return &QueryResult{
			Columns:  cols,
			Rows:     data,
			IsSelect: true,
		}, nil
	}
	res, err := sess.DB.ExecContext(ctx, sqlText)
	if err != nil {
		return nil, err
	}
	n, _ := res.RowsAffected()
	return &QueryResult{
		RowsAffected: n,
		IsSelect:     false,
	}, nil
}

func buildWhere(driver string, pk map[string]interface{}) (string, []interface{}, error) {
	parts := make([]string, 0, len(pk))
	args := make([]interface{}, 0, len(pk))
	for k, v := range pk {
		cq, err := quoteIdent(driver, k)
		if err != nil {
			return "", nil, err
		}
		if v == nil {
			parts = append(parts, cq+" IS NULL")
			continue
		}
		parts = append(parts, cq+" = ?")
		args = append(args, normalizeWrite(v))
	}
	return strings.Join(parts, " AND "), args, nil
}

func readRows(rows *sql.Rows, max int) ([]string, []map[string]interface{}, error) {
	cols, err := rows.Columns()
	if err != nil {
		return nil, nil, err
	}
	var data []map[string]interface{}
	for rows.Next() {
		if max > 0 && len(data) >= max {
			break
		}
		raw := make([]interface{}, len(cols))
		ptrs := make([]interface{}, len(cols))
		for i := range raw {
			ptrs[i] = &raw[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, nil, err
		}
		row := make(map[string]interface{}, len(cols))
		for i, c := range cols {
			row[c] = normalizeRead(raw[i])
		}
		data = append(data, row)
	}
	if data == nil {
		data = []map[string]interface{}{}
	}
	return cols, data, rows.Err()
}

func normalizeRead(v interface{}) interface{} {
	if v == nil {
		return nil
	}
	switch t := v.(type) {
	case []byte:
		return string(t)
	case time.Time:
		return t.Format(time.RFC3339Nano)
	default:
		return t
	}
}

func normalizeWrite(v interface{}) interface{} {
	if v == nil {
		return nil
	}
	switch t := v.(type) {
	case string:
		if t == "" {
			// keep empty string (not null); UI can send null explicitly
			return t
		}
		return t
	default:
		return v
	}
}

// HasPrimaryKey reports whether columns include at least one PK.
func HasPrimaryKey(cols []Column) bool {
	for _, c := range cols {
		if c.Primary {
			return true
		}
	}
	return false
}

// PrimaryKeyNames returns PK column names in order.
func PrimaryKeyNames(cols []Column) []string {
	var out []string
	for _, c := range cols {
		if c.Primary {
			out = append(out, c.Name)
		}
	}
	return out
}
