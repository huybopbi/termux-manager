package termux

import (
	"os"
	"path/filepath"
)

// QuickPath is a bookmark root the file manager can jump to.
type QuickPath struct {
	ID        string `json:"id"`
	Label     string `json:"label"`
	Path      string `json:"path"`
	Available bool   `json:"available"`
}

func dirOK(p string) bool {
	if p == "" {
		return false
	}
	fi, err := os.Stat(p)
	return err == nil && fi.IsDir()
}

// QuickPaths returns common Termux / Android locations (and Home everywhere).
func QuickPaths() []QuickPath {
	home, _ := os.UserHomeDir()
	var out []QuickPath

	add := func(id, label, path string) {
		if path == "" {
			return
		}
		clean := filepath.Clean(path)
		// Deduplicate by cleaned path
		for _, existing := range out {
			if filepath.Clean(existing.Path) == clean {
				return
			}
		}
		out = append(out, QuickPath{
			ID:        id,
			Label:     label,
			Path:      clean,
			Available: dirOK(clean),
		})
	}

	add("home", "Home", home)
	add("termux-data", "Termux Data", "/data/data/com.termux/files")

	if IsTermux() || dirOK("/sdcard") {
		add("sdcard", "Storage", "/sdcard")
		add("download", "Download", "/sdcard/Download")
		add("dcim", "DCIM", "/sdcard/DCIM")
		if home != "" {
			add("shared", "Shared", filepath.Join(home, "storage", "shared"))
		}
	}

	if prefix := os.Getenv("PREFIX"); prefix != "" {
		add("prefix", "Prefix", prefix)
	}

	return out
}

// IsAllowedRoot reports whether path is an available quick path.
func IsAllowedRoot(path string) bool {
	clean := filepath.Clean(path)
	for _, qp := range QuickPaths() {
		if qp.Available && filepath.Clean(qp.Path) == clean {
			return true
		}
	}
	return false
}
