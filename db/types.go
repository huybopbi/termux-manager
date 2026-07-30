package db

import "database/sql"

const (
	DriverMySQL  = "mysql"
	DriverSQLite = "sqlite"

	DefaultRowLimit = 100
	MaxRowLimit     = 500
	QueryTimeout    = 15 // seconds
)

type ConnectConfig struct {
	Driver   string `json:"driver"`
	Host     string `json:"host"`
	Port     int    `json:"port"`
	User     string `json:"user"`
	Password string `json:"password"`
	Database string `json:"database"`
	Path     string `json:"path"` // absolute SQLite path (resolved by server)
}

type Column struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Nullable bool   `json:"nullable"`
	Primary  bool   `json:"primary"`
	Default  *string `json:"default,omitempty"`
}

type TableInfo struct {
	Name string `json:"name"`
	Type string `json:"type"` // table | view
}

type RowsResult struct {
	Columns []string                 `json:"columns"`
	Rows    []map[string]interface{} `json:"rows"`
	Limit   int                      `json:"limit"`
	Offset  int                      `json:"offset"`
	HasMore bool                     `json:"has_more"`
}

type QueryResult struct {
	Columns      []string                 `json:"columns,omitempty"`
	Rows         []map[string]interface{} `json:"rows,omitempty"`
	RowsAffected int64                    `json:"rows_affected,omitempty"`
	IsSelect     bool                     `json:"is_select"`
}

type ConnectResult struct {
	SessionID string   `json:"session_id"`
	Driver    string   `json:"driver"`
	Database  string   `json:"database"`
	Databases []string `json:"databases,omitempty"`
}

type Session struct {
	ID       string
	Driver   string
	DB       *sql.DB
	Database string
	Path     string
}
