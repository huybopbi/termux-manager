package db

import (
	"testing"
)

func TestSQLiteCRUD(t *testing.T) {
	sess, err := Open(ConnectConfig{Driver: DriverSQLite, Path: ":memory:"})
	if err != nil {
		t.Fatal(err)
	}
	defer sess.DB.Close()

	store := NewStore(DefaultIdleTTL)
	defer store.Close()
	id := store.Put(sess)
	sess, err = store.Get(id)
	if err != nil {
		t.Fatal(err)
	}

	_, err = sess.DB.Exec(`CREATE TABLE users (
		id INTEGER PRIMARY KEY,
		name TEXT NOT NULL,
		age INTEGER
	)`)
	if err != nil {
		t.Fatal(err)
	}

	tables, err := ListTables(sess)
	if err != nil {
		t.Fatal(err)
	}
	if len(tables) != 1 || tables[0].Name != "users" {
		t.Fatalf("tables: %+v", tables)
	}

	cols, err := ListColumns(sess, "users")
	if err != nil {
		t.Fatal(err)
	}
	if !HasPrimaryKey(cols) {
		t.Fatal("expected PK")
	}

	if err := InsertRow(sess, "users", map[string]interface{}{
		"id": 1, "name": "alice", "age": 30,
	}); err != nil {
		t.Fatal(err)
	}
	if err := InsertRow(sess, "users", map[string]interface{}{
		"id": 2, "name": "bob", "age": 20,
	}); err != nil {
		t.Fatal(err)
	}

	rows, err := ListRows(sess, "users", 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows.Rows) != 2 {
		t.Fatalf("want 2 rows, got %d", len(rows.Rows))
	}

	if err := UpdateRow(sess, "users",
		map[string]interface{}{"id": 1},
		map[string]interface{}{"name": "ally", "age": 31},
	); err != nil {
		t.Fatal(err)
	}

	q, err := ExecQuery(sess, "SELECT name FROM users WHERE id = 1")
	if err != nil {
		t.Fatal(err)
	}
	if !q.IsSelect || len(q.Rows) != 1 || q.Rows[0]["name"] != "ally" {
		t.Fatalf("query: %+v", q)
	}

	if err := DeleteRow(sess, "users", map[string]interface{}{"id": 2}); err != nil {
		t.Fatal(err)
	}
	rows, err = ListRows(sess, "users", 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows.Rows) != 1 {
		t.Fatalf("want 1 row after delete, got %d", len(rows.Rows))
	}

	exec, err := ExecQuery(sess, "UPDATE users SET age = 99 WHERE id = 1")
	if err != nil {
		t.Fatal(err)
	}
	if exec.IsSelect || exec.RowsAffected != 1 {
		t.Fatalf("exec: %+v", exec)
	}

	if err := store.Remove(id); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Get(id); err == nil {
		t.Fatal("expected session gone")
	}
}
