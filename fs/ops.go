package fs

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type FileInfo struct {
	Name    string    `json:"name"`
	Path    string    `json:"path"`
	Size    int64     `json:"size"`
	IsDir   bool      `json:"is_dir"`
	ModTime time.Time `json:"mod_time"`
	Mode    string    `json:"mode"`
	Ext     string    `json:"ext"`
}

type ListResult struct {
	Path    string     `json:"path"`
	Parent  string     `json:"parent"`
	Files   []FileInfo `json:"files"`
}

func List(root, relPath string, showHidden bool) (*ListResult, error) {
	absPath := filepath.Join(root, relPath)
	absPath = filepath.Clean(absPath)

	// Security: ensure we stay within root
	if !strings.HasPrefix(absPath, root) {
		absPath = root
		relPath = ""
	}

	entries, err := os.ReadDir(absPath)
	if err != nil {
		return nil, err
	}

	var files []FileInfo
	for _, e := range entries {
		if !showHidden && strings.HasPrefix(e.Name(), ".") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		rel, _ := filepath.Rel(root, filepath.Join(absPath, e.Name()))
		ext := ""
		if !e.IsDir() {
			ext = strings.TrimPrefix(filepath.Ext(e.Name()), ".")
		}
		files = append(files, FileInfo{
			Name:    e.Name(),
			Path:    rel,
			Size:    info.Size(),
			IsDir:   e.IsDir(),
			ModTime: info.ModTime(),
			Mode:    info.Mode().String(),
			Ext:     ext,
		})
	}

	// Dirs first, then files, each alpha
	sort.Slice(files, func(i, j int) bool {
		if files[i].IsDir != files[j].IsDir {
			return files[i].IsDir
		}
		return strings.ToLower(files[i].Name) < strings.ToLower(files[j].Name)
	})

	parent := filepath.Dir(relPath)
	if parent == "." {
		parent = ""
	}

	return &ListResult{
		Path:   relPath,
		Parent: parent,
		Files:  files,
	}, nil
}

func Delete(root, relPath string) error {
	abs := safeJoin(root, relPath)
	if abs == "" {
		return os.ErrInvalid
	}
	return os.RemoveAll(abs)
}

func Rename(root, oldRel, newName string) error {
	oldAbs := safeJoin(root, oldRel)
	if oldAbs == "" {
		return os.ErrInvalid
	}
	newAbs := filepath.Join(filepath.Dir(oldAbs), newName)
	// Ensure new path still within root
	if !strings.HasPrefix(newAbs, root) {
		return os.ErrInvalid
	}
	return os.Rename(oldAbs, newAbs)
}

func Move(root, srcRel, dstRel string) error {
	src := safeJoin(root, srcRel)
	dst := safeJoin(root, dstRel)
	if src == "" || dst == "" {
		return os.ErrInvalid
	}
	return os.Rename(src, dst)
}

func Copy(root, srcRel, dstRel string) error {
	src := safeJoin(root, srcRel)
	dst := safeJoin(root, dstRel)
	if src == "" || dst == "" {
		return os.ErrInvalid
	}
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return copyDir(src, dst)
	}
	return copyFile(src, dst)
}

func Mkdir(root, relPath string) error {
	abs := safeJoin(root, relPath)
	if abs == "" {
		return os.ErrInvalid
	}
	return os.MkdirAll(abs, 0755)
}

func CreateFile(root, relPath string) error {
	abs := safeJoin(root, relPath)
	if abs == "" {
		return os.ErrInvalid
	}
	f, err := os.Create(abs)
	if err != nil {
		return err
	}
	return f.Close()
}

func ReadFile(root, relPath string) ([]byte, error) {
	abs := safeJoin(root, relPath)
	if abs == "" {
		return nil, os.ErrInvalid
	}
	return os.ReadFile(abs)
}

func WriteFile(root, relPath string, data []byte) error {
	abs := safeJoin(root, relPath)
	if abs == "" {
		return os.ErrInvalid
	}
	return os.WriteFile(abs, data, 0644)
}

func Search(root, relPath, query string, showHidden bool) ([]FileInfo, error) {
	absPath := safeJoin(root, relPath)
	if absPath == "" {
		absPath = root
	}
	query = strings.ToLower(query)
	var results []FileInfo
	err := filepath.Walk(absPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // skip errors, keep going
		}
		name := info.Name()
		if !showHidden && strings.HasPrefix(name, ".") {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.Contains(strings.ToLower(name), query) {
			rel, _ := filepath.Rel(root, path)
			ext := ""
			if !info.IsDir() {
				ext = strings.TrimPrefix(filepath.Ext(name), ".")
			}
			results = append(results, FileInfo{
				Name:    name,
				Path:    rel,
				Size:    info.Size(),
				IsDir:   info.IsDir(),
				ModTime: info.ModTime(),
				Mode:    info.Mode().String(),
				Ext:     ext,
			})
		}
		return nil
	})
	return results, err
}

func Zip(root, relDir string, names []string, outName string) error {
	dir := safeJoin(root, relDir)
	if dir == "" {
		dir = root
	}
	outPath := filepath.Join(dir, outName)
	f, err := os.Create(outPath)
	if err != nil {
		return err
	}
	defer f.Close()
	w := zip.NewWriter(f)
	defer w.Close()
	for _, name := range names {
		src := filepath.Join(dir, name)
		if err := addToZip(w, src, name); err != nil {
			return err
		}
	}
	return nil
}

func Unzip(root, relPath, destRel string) error {
	src := safeJoin(root, relPath)
	dst := safeJoin(root, destRel)
	if src == "" || dst == "" {
		return os.ErrInvalid
	}
	r, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer r.Close()
	for _, f := range r.File {
		target := filepath.Join(dst, f.Name)
		if !strings.HasPrefix(target, dst) {
			continue // zip slip guard
		}
		if f.FileInfo().IsDir() {
			os.MkdirAll(target, 0755)
			continue
		}
		os.MkdirAll(filepath.Dir(target), 0755)
		rc, err := f.Open()
		if err != nil {
			return err
		}
		out, err := os.Create(target)
		if err != nil {
			rc.Close()
			return err
		}
		io.Copy(out, rc)
		out.Close()
		rc.Close()
	}
	return nil
}

func TarGz(root, relDir string, names []string, outName string) error {
	dir := safeJoin(root, relDir)
	if dir == "" {
		dir = root
	}
	outPath := filepath.Join(dir, outName)
	f, err := os.Create(outPath)
	if err != nil {
		return err
	}
	defer f.Close()
	gz := gzip.NewWriter(f)
	defer gz.Close()
	tw := tar.NewWriter(gz)
	defer tw.Close()
	for _, name := range names {
		src := filepath.Join(dir, name)
		if err := addToTar(tw, src, name); err != nil {
			return err
		}
	}
	return nil
}

// Untar extracts a .tar, .tar.gz, or .tgz archive into destRel.
func Untar(root, relPath, destRel string) error {
	src := safeJoin(root, relPath)
	dst := safeJoin(root, destRel)
	if src == "" || dst == "" {
		return os.ErrInvalid
	}
	f, err := os.Open(src)
	if err != nil {
		return err
	}
	defer f.Close()

	var r io.Reader = f
	lower := strings.ToLower(src)
	if strings.HasSuffix(lower, ".tar.gz") || strings.HasSuffix(lower, ".tgz") {
		gz, err := gzip.NewReader(f)
		if err != nil {
			return err
		}
		defer gz.Close()
		r = gz
	}

	tr := tar.NewReader(r)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		name := filepath.Clean(filepath.FromSlash(hdr.Name))
		if name == "." || name == "" {
			continue
		}
		if name == ".." || strings.HasPrefix(name, ".."+string(os.PathSeparator)) {
			continue
		}
		target := filepath.Join(dst, name)
		if rel, err := filepath.Rel(dst, target); err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
			continue // tar slip guard
		}

		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0755); err != nil {
				return err
			}
		case tar.TypeReg, tar.TypeRegA:
			if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
				return err
			}
			mode := os.FileMode(hdr.Mode) & 0777
			if mode == 0 {
				mode = 0644
			}
			out, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
			if err != nil {
				return err
			}
			if _, err := io.Copy(out, tr); err != nil {
				out.Close()
				return err
			}
			out.Close()
		default:
			// skip symlinks / specials
		}
	}
	return nil
}

// ArchiveDestName returns a default extract directory name for an archive path.
func ArchiveDestName(relPath string) string {
	lower := strings.ToLower(relPath)
	for _, suf := range []string{".tar.gz", ".tgz", ".tar.bz2", ".tar.xz"} {
		if strings.HasSuffix(lower, suf) {
			return relPath[:len(relPath)-len(suf)]
		}
	}
	return strings.TrimSuffix(relPath, filepath.Ext(relPath))
}

// --- helpers ---

func safeJoin(root, rel string) string {
	rel = filepath.Clean("/" + rel)
	abs := filepath.Join(root, rel)
	if !strings.HasPrefix(abs, root) {
		return ""
	}
	return abs
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}

func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, path)
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, info.Mode())
		}
		return copyFile(path, target)
	})
}

func addToZip(w *zip.Writer, src, name string) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return filepath.Walk(src, func(path string, fi os.FileInfo, err error) error {
			if err != nil || fi.IsDir() {
				return err
			}
			rel, _ := filepath.Rel(filepath.Dir(src), path)
			f, err := w.Create(rel)
			if err != nil {
				return err
			}
			in, err := os.Open(path)
			if err != nil {
				return err
			}
			defer in.Close()
			_, err = io.Copy(f, in)
			return err
		})
	}
	f, err := w.Create(name)
	if err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	_, err = io.Copy(f, in)
	return err
}

func addToTar(tw *tar.Writer, src, name string) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return filepath.Walk(src, func(path string, fi os.FileInfo, err error) error {
			if err != nil || fi.IsDir() {
				return err
			}
			rel, _ := filepath.Rel(filepath.Dir(src), path)
			hdr := &tar.Header{Name: rel, Size: fi.Size(), Mode: int64(fi.Mode()), ModTime: fi.ModTime()}
			if err := tw.WriteHeader(hdr); err != nil {
				return err
			}
			in, err := os.Open(path)
			if err != nil {
				return err
			}
			defer in.Close()
			_, err = io.Copy(tw, in)
			return err
		})
	}
	hdr := &tar.Header{Name: name, Size: info.Size(), Mode: int64(info.Mode()), ModTime: info.ModTime()}
	if err := tw.WriteHeader(hdr); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	_, err = io.Copy(tw, in)
	return err
}
