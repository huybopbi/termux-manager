package db

import (
	"database/sql"
	"fmt"
	"strings"

	_ "github.com/go-sql-driver/mysql"
	_ "modernc.org/sqlite"
)

func Open(cfg ConnectConfig) (*Session, error) {
	driver := strings.ToLower(strings.TrimSpace(cfg.Driver))
	switch driver {
	case DriverMySQL:
		return openMySQL(cfg)
	case DriverSQLite, "sqlite3":
		return openSQLite(cfg)
	default:
		return nil, fmt.Errorf("unsupported driver %q (use mysql or sqlite)", cfg.Driver)
	}
}

func openMySQL(cfg ConnectConfig) (*Session, error) {
	host := cfg.Host
	if host == "" {
		host = "127.0.0.1"
	}
	port := cfg.Port
	if port <= 0 {
		port = 3306
	}
	user := cfg.User
	if user == "" {
		user = "root"
	}
	dbName := cfg.Database
	// Allow connect without database; user picks later.
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?parseTime=true&charset=utf8mb4&loc=Local",
		user, cfg.Password, host, port, dbName)
	sqlDB, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxOpenConns(4)
	sqlDB.SetMaxIdleConns(2)
	if err := sqlDB.Ping(); err != nil {
		_ = sqlDB.Close()
		return nil, err
	}
	return &Session{
		Driver:   DriverMySQL,
		DB:       sqlDB,
		Database: dbName,
	}, nil
}

func openSQLite(cfg ConnectConfig) (*Session, error) {
	path := strings.TrimSpace(cfg.Path)
	if path == "" {
		return nil, fmt.Errorf("sqlite path required")
	}
	if path != ":memory:" {
		// file must exist for open (we don't create empty DBs via connect)
		dsn := "file:" + path + "?_pragma=foreign_keys(1)"
		sqlDB, err := sql.Open("sqlite", dsn)
		if err != nil {
			return nil, err
		}
		sqlDB.SetMaxOpenConns(1)
		if err := sqlDB.Ping(); err != nil {
			_ = sqlDB.Close()
			return nil, err
		}
		return &Session{
			Driver:   DriverSQLite,
			DB:       sqlDB,
			Database: path,
			Path:     path,
		}, nil
	}
	sqlDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		return nil, err
	}
	if err := sqlDB.Ping(); err != nil {
		_ = sqlDB.Close()
		return nil, err
	}
	return &Session{
		Driver:   DriverSQLite,
		DB:       sqlDB,
		Database: ":memory:",
		Path:     ":memory:",
	}, nil
}
