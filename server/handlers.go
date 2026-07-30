package server

import (
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	fsops "github.com/huybopbi/termux-manager/fs"
	"github.com/huybopbi/termux-manager/termux"
)

type Server struct {
	mu          sync.RWMutex
	Root        string
	InitialRoot string
	ShowHidden  bool
}

func (s *Server) quickPaths() []termux.QuickPath {
	paths := termux.QuickPaths()
	ensure := func(id, label, path string) {
		if path == "" {
			return
		}
		clean := filepath.Clean(path)
		for _, p := range paths {
			if filepath.Clean(p.Path) == clean {
				return
			}
		}
		fi, err := os.Stat(clean)
		if err != nil || !fi.IsDir() {
			return
		}
		paths = append([]termux.QuickPath{{
			ID: id, Label: label, Path: clean, Available: true,
		}}, paths...)
	}
	s.mu.RLock()
	initial, current := s.InitialRoot, s.Root
	s.mu.RUnlock()
	ensure("start", "Start", initial)
	ensure("root", "Root", current)
	return paths
}

func (s *Server) canUseRoot(path string) bool {
	clean := filepath.Clean(path)
	for _, p := range s.quickPaths() {
		if p.Available && filepath.Clean(p.Path) == clean {
			return true
		}
	}
	return false
}

func (s *Server) rootPath() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.Root
}

func (s *Server) setRoot(path string) {
	s.mu.Lock()
	s.Root = path
	s.mu.Unlock()
}

type apiResponse struct {
	OK    bool        `json:"ok"`
	Data  interface{} `json:"data,omitempty"`
	Error string      `json:"error,omitempty"`
}

func (s *Server) respond(w http.ResponseWriter, status int, data interface{}, err error) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err != nil {
		json.NewEncoder(w).Encode(apiResponse{OK: false, Error: err.Error()})
	} else {
		json.NewEncoder(w).Encode(apiResponse{OK: true, Data: data})
	}
}

func (s *Server) ok(w http.ResponseWriter, data interface{}) {
	s.respond(w, http.StatusOK, data, nil)
}

func (s *Server) fail(w http.ResponseWriter, status int, err error) {
	s.respond(w, status, nil, err)
}

// GET /api/list?path=foo/bar
func (s *Server) handleList(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	result, err := fsops.List(s.rootPath(), path, s.ShowHidden)
	if err != nil {
		s.fail(w, http.StatusBadRequest, err)
		return
	}
	s.ok(w, result)
}

// GET /api/read?path=foo/bar.txt
func (s *Server) handleRead(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	data, err := fsops.ReadFile(s.rootPath(), path)
	if err != nil {
		s.fail(w, http.StatusBadRequest, err)
		return
	}
	s.ok(w, string(data))
}

// POST /api/write?path=foo/bar.txt  body: raw text
func (s *Server) handleWrite(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	data, err := io.ReadAll(r.Body)
	if err != nil {
		s.fail(w, http.StatusBadRequest, err)
		return
	}
	if err := fsops.WriteFile(s.rootPath(), path, data); err != nil {
		s.fail(w, http.StatusInternalServerError, err)
		return
	}
	s.ok(w, nil)
}

// DELETE /api/delete  body: {"paths":["a","b"]}
func (s *Server) handleDelete(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Paths []string `json:"paths"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.fail(w, http.StatusBadRequest, err)
		return
	}
	var errs []string
	for _, p := range req.Paths {
		if err := fsops.Delete(s.rootPath(), p); err != nil {
			errs = append(errs, fmt.Sprintf("%s: %s", p, err))
		}
	}
	if len(errs) > 0 {
		s.fail(w, http.StatusInternalServerError, fmt.Errorf(strings.Join(errs, "; ")))
		return
	}
	s.ok(w, nil)
}

// POST /api/rename  body: {"path":"old","name":"newname"}
func (s *Server) handleRename(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Path string `json:"path"`
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.fail(w, http.StatusBadRequest, err)
		return
	}
	if err := fsops.Rename(s.rootPath(), req.Path, req.Name); err != nil {
		s.fail(w, http.StatusInternalServerError, err)
		return
	}
	s.ok(w, nil)
}

// POST /api/move  body: {"src":"path/a","dst":"path/b"}
func (s *Server) handleMove(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Src string `json:"src"`
		Dst string `json:"dst"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.fail(w, http.StatusBadRequest, err)
		return
	}
	if err := fsops.Move(s.rootPath(), req.Src, req.Dst); err != nil {
		s.fail(w, http.StatusInternalServerError, err)
		return
	}
	s.ok(w, nil)
}

// POST /api/copy  body: {"src":"path/a","dst":"path/b"}
func (s *Server) handleCopy(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Src string `json:"src"`
		Dst string `json:"dst"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.fail(w, http.StatusBadRequest, err)
		return
	}
	if err := fsops.Copy(s.rootPath(), req.Src, req.Dst); err != nil {
		s.fail(w, http.StatusInternalServerError, err)
		return
	}
	s.ok(w, nil)
}

// POST /api/mkdir  body: {"path":"new/dir"}
func (s *Server) handleMkdir(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Path string `json:"path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.fail(w, http.StatusBadRequest, err)
		return
	}
	if err := fsops.Mkdir(s.rootPath(), req.Path); err != nil {
		s.fail(w, http.StatusInternalServerError, err)
		return
	}
	s.ok(w, nil)
}

// POST /api/touch  body: {"path":"new/file.txt"}
func (s *Server) handleTouch(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Path string `json:"path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.fail(w, http.StatusBadRequest, err)
		return
	}
	if err := fsops.CreateFile(s.rootPath(), req.Path); err != nil {
		s.fail(w, http.StatusInternalServerError, err)
		return
	}
	s.ok(w, nil)
}

// GET /api/search?path=foo&q=query
func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	query := r.URL.Query().Get("q")
	if query == "" {
		s.fail(w, http.StatusBadRequest, fmt.Errorf("query is empty"))
		return
	}
	results, err := fsops.Search(s.rootPath(), path, query, s.ShowHidden)
	if err != nil {
		s.fail(w, http.StatusInternalServerError, err)
		return
	}
	s.ok(w, results)
}

// GET /api/download?path=foo/bar.txt[&inline=1]
func (s *Server) handleDownload(w http.ResponseWriter, r *http.Request) {
	root := s.rootPath()
	rel := r.URL.Query().Get("path")
	abs := filepath.Join(root, filepath.Clean("/"+rel))
	if !strings.HasPrefix(abs, root) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	info, err := os.Stat(abs)
	if err != nil || info.IsDir() {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	ext := filepath.Ext(abs)
	mimeType := mime.TypeByExtension(ext)
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}
	disp := "attachment"
	if r.URL.Query().Get("inline") == "1" {
		disp = "inline"
	}
	w.Header().Set("Content-Disposition", fmt.Sprintf(`%s; filename="%s"`, disp, info.Name()))
	w.Header().Set("Content-Type", mimeType)
	http.ServeFile(w, r, abs)
}

// POST /api/upload?path=dir   multipart/form-data field "file"
func (s *Server) handleUpload(w http.ResponseWriter, r *http.Request) {
	dirRel := r.URL.Query().Get("path")
	dirAbs := filepath.Join(s.rootPath(), filepath.Clean("/"+dirRel))
	if !strings.HasPrefix(dirAbs, s.rootPath()) {
		s.fail(w, http.StatusForbidden, fmt.Errorf("forbidden"))
		return
	}

	r.ParseMultipartForm(512 << 20) // 512 MB max
	file, header, err := r.FormFile("file")
	if err != nil {
		s.fail(w, http.StatusBadRequest, err)
		return
	}
	defer file.Close()

	dest := filepath.Join(dirAbs, filepath.Base(header.Filename))
	out, err := os.Create(dest)
	if err != nil {
		s.fail(w, http.StatusInternalServerError, err)
		return
	}
	defer out.Close()
	io.Copy(out, file)
	s.ok(w, map[string]string{"name": header.Filename})
}

// POST /api/zip  body: {"path":"dir","files":["a","b"],"name":"archive.zip"}
func (s *Server) handleZip(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Path  string   `json:"path"`
		Files []string `json:"files"`
		Name  string   `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.fail(w, http.StatusBadRequest, err)
		return
	}
	if req.Name == "" {
		req.Name = fmt.Sprintf("archive_%s.zip", time.Now().Format("20060102_150405"))
	}
	if err := fsops.Zip(s.rootPath(), req.Path, req.Files, req.Name); err != nil {
		s.fail(w, http.StatusInternalServerError, err)
		return
	}
	s.ok(w, map[string]string{"name": req.Name})
}

// POST /api/unzip  body: {"path":"archive.zip","dest":"output_dir"}
func (s *Server) handleUnzip(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Path string `json:"path"`
		Dest string `json:"dest"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.fail(w, http.StatusBadRequest, err)
		return
	}
	if req.Dest == "" {
		req.Dest = strings.TrimSuffix(req.Path, filepath.Ext(req.Path))
	}
	if err := fsops.Unzip(s.rootPath(), req.Path, req.Dest); err != nil {
		s.fail(w, http.StatusInternalServerError, err)
		return
	}
	s.ok(w, nil)
}

// POST /api/tar  body: {"path":"dir","files":["a","b"],"name":"archive.tar.gz"}
func (s *Server) handleTar(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Path  string   `json:"path"`
		Files []string `json:"files"`
		Name  string   `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.fail(w, http.StatusBadRequest, err)
		return
	}
	if req.Name == "" {
		req.Name = fmt.Sprintf("archive_%s.tar.gz", time.Now().Format("20060102_150405"))
	}
	if err := fsops.TarGz(s.rootPath(), req.Path, req.Files, req.Name); err != nil {
		s.fail(w, http.StatusInternalServerError, err)
		return
	}
	s.ok(w, map[string]string{"name": req.Name})
}

// POST /api/termux/share  body: {"path":"file"}
func (s *Server) handleTermuxShare(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Path string `json:"path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.fail(w, http.StatusBadRequest, err)
		return
	}
	abs := filepath.Join(s.rootPath(), filepath.Clean("/"+req.Path))
	if err := termux.ShareFile(abs); err != nil {
		s.fail(w, http.StatusInternalServerError, err)
		return
	}
	s.ok(w, nil)
}

// POST /api/termux/clipboard  body: {"text":"..."}
func (s *Server) handleTermuxClipboard(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Text string `json:"text"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.fail(w, http.StatusBadRequest, err)
		return
	}
	if err := termux.CopyToClipboard(req.Text); err != nil {
		s.fail(w, http.StatusInternalServerError, err)
		return
	}
	s.ok(w, nil)
}

// POST /api/termux/exec  body: {"cmd":"ls -la"}
func (s *Server) handleTermuxExec(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Cmd string `json:"cmd"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.fail(w, http.StatusBadRequest, err)
		return
	}
	out, err := termux.RunCommand(req.Cmd)
	if err != nil {
		// Return output even on non-zero exit (useful for showing error output)
		s.ok(w, map[string]interface{}{"output": out, "error": err.Error()})
		return
	}
	s.ok(w, map[string]string{"output": out})
}

// GET /api/info
func (s *Server) handleInfo(w http.ResponseWriter, r *http.Request) {
	root := s.rootPath()
	s.ok(w, map[string]interface{}{
		"root":        root,
		"is_termux":   termux.IsTermux(),
		"has_storage": termux.HasStorageAccess(),
		"show_hidden": s.ShowHidden,
		"quick_paths": s.quickPaths(),
	})
}

// POST /api/root  body: {"path":"/sdcard"}
func (s *Server) handleSetRoot(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Path string `json:"path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.fail(w, http.StatusBadRequest, err)
		return
	}
	path := filepath.Clean(req.Path)
	if path == "" || path == "." {
		s.fail(w, http.StatusBadRequest, fmt.Errorf("path required"))
		return
	}
	if !s.canUseRoot(path) {
		s.fail(w, http.StatusForbidden, fmt.Errorf("path not in quick paths"))
		return
	}
	fi, err := os.Stat(path)
	if err != nil || !fi.IsDir() {
		s.fail(w, http.StatusBadRequest, fmt.Errorf("directory not found"))
		return
	}
	s.setRoot(path)
	s.ok(w, map[string]interface{}{
		"root":        path,
		"quick_paths": s.quickPaths(),
	})
}
