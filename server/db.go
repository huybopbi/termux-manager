package server

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"

	dbpkg "github.com/huybopbi/termux-manager/db"
	fsops "github.com/huybopbi/termux-manager/fs"
)

const dbSessionHeader = "X-DB-Session"

var (
	dbStoreOnce sync.Once
	dbStore     *dbpkg.Store
)

func (s *Server) dbSessions() *dbpkg.Store {
	dbStoreOnce.Do(func() {
		dbStore = dbpkg.NewStore(dbpkg.DefaultIdleTTL)
	})
	return dbStore
}

func (s *Server) sessionID(r *http.Request) string {
	if id := r.Header.Get(dbSessionHeader); id != "" {
		return id
	}
	return r.URL.Query().Get("session")
}

func (s *Server) requireDBSession(w http.ResponseWriter, r *http.Request) (*dbpkg.Session, bool) {
	id := s.sessionID(r)
	if id == "" {
		s.fail(w, http.StatusBadRequest, fmt.Errorf("missing %s header", dbSessionHeader))
		return nil, false
	}
	sess, err := s.dbSessions().Get(id)
	if err != nil {
		s.fail(w, http.StatusUnauthorized, err)
		return nil, false
	}
	return sess, true
}

func (s *Server) decodeJSON(w http.ResponseWriter, r *http.Request, dst interface{}) bool {
	defer r.Body.Close()
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		s.fail(w, http.StatusBadRequest, err)
		return false
	}
	if len(body) == 0 {
		s.fail(w, http.StatusBadRequest, fmt.Errorf("empty body"))
		return false
	}
	if err := json.Unmarshal(body, dst); err != nil {
		s.fail(w, http.StatusBadRequest, err)
		return false
	}
	return true
}

// POST /api/db/connect
func (s *Server) handleDBConnect(w http.ResponseWriter, r *http.Request) {
	var req dbpkg.ConnectConfig
	if !s.decodeJSON(w, r, &req) {
		return
	}
	driver := strings.ToLower(strings.TrimSpace(req.Driver))
	if driver == "sqlite" || driver == "sqlite3" {
		abs, err := fsops.AbsPath(s.rootPath(), req.Path)
		if err != nil {
			// allow :memory: only in tests — reject for API
			if strings.TrimSpace(req.Path) == ":memory:" {
				s.fail(w, http.StatusBadRequest, fmt.Errorf("memory databases not allowed via API"))
				return
			}
			s.fail(w, http.StatusBadRequest, err)
			return
		}
		req.Path = abs
		req.Driver = dbpkg.DriverSQLite
	} else {
		req.Driver = dbpkg.DriverMySQL
	}

	sess, err := dbpkg.Open(req)
	if err != nil {
		s.fail(w, http.StatusBadRequest, err)
		return
	}
	id := s.dbSessions().Put(sess)
	sess, _ = s.dbSessions().Get(id)

	result := dbpkg.ConnectResult{
		SessionID: id,
		Driver:    sess.Driver,
		Database:  sess.Database,
	}
	if dbs, err := dbpkg.ListDatabases(sess); err == nil {
		result.Databases = dbs
	}
	s.ok(w, result)
}

// POST /api/db/disconnect
func (s *Server) handleDBDisconnect(w http.ResponseWriter, r *http.Request) {
	id := s.sessionID(r)
	if id == "" {
		s.fail(w, http.StatusBadRequest, fmt.Errorf("missing session"))
		return
	}
	if err := s.dbSessions().Remove(id); err != nil {
		s.fail(w, http.StatusBadRequest, err)
		return
	}
	s.ok(w, nil)
}

// GET /api/db/databases
func (s *Server) handleDBDatabases(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.requireDBSession(w, r)
	if !ok {
		return
	}
	dbs, err := dbpkg.ListDatabases(sess)
	if err != nil {
		s.fail(w, http.StatusBadRequest, err)
		return
	}
	s.ok(w, dbs)
}

// POST /api/db/use
func (s *Server) handleDBUse(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.requireDBSession(w, r)
	if !ok {
		return
	}
	var req struct {
		Database string `json:"database"`
	}
	if !s.decodeJSON(w, r, &req) {
		return
	}
	if strings.TrimSpace(req.Database) == "" {
		s.fail(w, http.StatusBadRequest, fmt.Errorf("database required"))
		return
	}
	if err := dbpkg.UseDatabase(sess, s.dbSessions(), req.Database); err != nil {
		s.fail(w, http.StatusBadRequest, err)
		return
	}
	s.ok(w, map[string]string{"database": req.Database})
}

// GET /api/db/tables
func (s *Server) handleDBTables(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.requireDBSession(w, r)
	if !ok {
		return
	}
	tables, err := dbpkg.ListTables(sess)
	if err != nil {
		s.fail(w, http.StatusBadRequest, err)
		return
	}
	s.ok(w, tables)
}

// GET /api/db/columns?table=
func (s *Server) handleDBColumns(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.requireDBSession(w, r)
	if !ok {
		return
	}
	table := r.URL.Query().Get("table")
	if table == "" {
		s.fail(w, http.StatusBadRequest, fmt.Errorf("table required"))
		return
	}
	cols, err := dbpkg.ListColumns(sess, table)
	if err != nil {
		s.fail(w, http.StatusBadRequest, err)
		return
	}
	s.ok(w, cols)
}

// GET /api/db/rows?table=&limit=&offset=
func (s *Server) handleDBRows(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.requireDBSession(w, r)
	if !ok {
		return
	}
	table := r.URL.Query().Get("table")
	if table == "" {
		s.fail(w, http.StatusBadRequest, fmt.Errorf("table required"))
		return
	}
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	result, err := dbpkg.ListRows(sess, table, limit, offset)
	if err != nil {
		s.fail(w, http.StatusBadRequest, err)
		return
	}
	s.ok(w, result)
}

// POST|PUT|DELETE /api/db/row
func (s *Server) handleDBRow(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Methods", "POST, PUT, DELETE, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type, "+dbSessionHeader)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	sess, ok := s.requireDBSession(w, r)
	if !ok {
		return
	}
	var req struct {
		Table  string                 `json:"table"`
		Values map[string]interface{} `json:"values"`
		PK     map[string]interface{} `json:"pk"`
	}
	if !s.decodeJSON(w, r, &req) {
		return
	}
	if strings.TrimSpace(req.Table) == "" {
		s.fail(w, http.StatusBadRequest, fmt.Errorf("table required"))
		return
	}
	var err error
	switch r.Method {
	case http.MethodPost:
		err = dbpkg.InsertRow(sess, req.Table, req.Values)
	case http.MethodPut:
		err = dbpkg.UpdateRow(sess, req.Table, req.PK, req.Values)
	case http.MethodDelete:
		err = dbpkg.DeleteRow(sess, req.Table, req.PK)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err != nil {
		s.fail(w, http.StatusBadRequest, err)
		return
	}
	s.ok(w, nil)
}

// POST /api/db/query
func (s *Server) handleDBQuery(w http.ResponseWriter, r *http.Request) {
	sess, ok := s.requireDBSession(w, r)
	if !ok {
		return
	}
	var req struct {
		SQL string `json:"sql"`
	}
	if !s.decodeJSON(w, r, &req) {
		return
	}
	result, err := dbpkg.ExecQuery(sess, req.SQL)
	if err != nil {
		s.fail(w, http.StatusBadRequest, err)
		return
	}
	s.ok(w, result)
}
