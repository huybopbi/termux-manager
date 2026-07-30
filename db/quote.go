package db

import (
	"fmt"
	"strings"
)

func quoteIdent(driver, name string) (string, error) {
	if name == "" {
		return "", fmt.Errorf("empty identifier")
	}
	if strings.ContainsAny(name, "\x00\n\r\t") {
		return "", fmt.Errorf("invalid identifier")
	}
	switch driver {
	case DriverMySQL:
		if strings.Contains(name, "`") {
			return "", fmt.Errorf("invalid identifier")
		}
		return "`" + name + "`", nil
	case DriverSQLite:
		if strings.Contains(name, "\"") {
			return "", fmt.Errorf("invalid identifier")
		}
		return `"` + name + `"`, nil
	default:
		return "", fmt.Errorf("unknown driver")
	}
}

func clampLimit(limit int) int {
	if limit <= 0 {
		return DefaultRowLimit
	}
	if limit > MaxRowLimit {
		return MaxRowLimit
	}
	return limit
}

func isSelectSQL(sqlText string) bool {
	s := strings.TrimSpace(sqlText)
	if s == "" {
		return false
	}
	// strip leading comments roughly
	for {
		s = strings.TrimSpace(s)
		if strings.HasPrefix(s, "--") {
			if i := strings.IndexByte(s, '\n'); i >= 0 {
				s = s[i+1:]
				continue
			}
			return false
		}
		if strings.HasPrefix(s, "/*") {
			if i := strings.Index(s, "*/"); i >= 0 {
				s = s[i+2:]
				continue
			}
			return false
		}
		break
	}
	upper := strings.ToUpper(s)
	return strings.HasPrefix(upper, "SELECT") ||
		strings.HasPrefix(upper, "SHOW") ||
		strings.HasPrefix(upper, "DESCRIBE") ||
		strings.HasPrefix(upper, "DESC ") ||
		strings.HasPrefix(upper, "EXPLAIN") ||
		strings.HasPrefix(upper, "PRAGMA") ||
		strings.HasPrefix(upper, "WITH")
}
