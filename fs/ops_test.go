package fs

import (
	"os"
	"path/filepath"
	"testing"
)

func TestUntarGz(t *testing.T) {
	root := t.TempDir()
	srcDir := filepath.Join(root, "pkg")
	if err := os.MkdirAll(srcDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "hello.txt"), []byte("hello tar"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := TarGz(root, "", []string{"pkg"}, "out.tar.gz"); err != nil {
		t.Fatal(err)
	}
	if err := Untar(root, "out.tar.gz", "extracted"); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(root, "extracted", "pkg", "hello.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "hello tar" {
		t.Fatalf("got %q", got)
	}
	if ArchiveDestName("foo/bar.tar.gz") != "foo/bar" {
		t.Fatalf("ArchiveDestName: %q", ArchiveDestName("foo/bar.tar.gz"))
	}
}
