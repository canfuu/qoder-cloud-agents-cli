package daemon

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSafePath(t *testing.T) {
	workdir := t.TempDir()

	w := &Worker{workdir: workdir}

	tests := []struct {
		name    string
		path    string
		wantErr bool
	}{
		{"relative file", "foo.txt", false},
		{"nested relative", "sub/dir/file.txt", false},
		{"absolute inside", filepath.Join(workdir, "inside.txt"), false},
		{"absolute outside", "/etc/passwd", true},
		{"traversal", "../../../etc/passwd", true},
		{"dot-dot in middle", "foo/../../etc/passwd", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := w.safePath(tt.path)
			if tt.wantErr && err == nil {
				t.Errorf("expected error for path %q", tt.path)
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error for path %q: %v", tt.path, err)
			}
		})
	}
}

func TestExecWrite(t *testing.T) {
	workdir := t.TempDir()
	w := &Worker{workdir: workdir}

	input := map[string]interface{}{
		"file_path": "test.txt",
		"content":   "hello world",
	}

	result, isErr := w.execWrite(input)
	if isErr {
		t.Fatalf("execWrite returned error: %s", result)
	}

	data, err := os.ReadFile(filepath.Join(workdir, "test.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "hello world" {
		t.Fatalf("expected 'hello world', got %q", string(data))
	}
}

func TestExecRead(t *testing.T) {
	workdir := t.TempDir()
	w := &Worker{workdir: workdir}

	os.WriteFile(filepath.Join(workdir, "read.txt"), []byte("content here"), 0644)

	input := map[string]interface{}{
		"file_path": "read.txt",
	}

	result, isErr := w.execRead(input)
	if isErr {
		t.Fatalf("execRead returned error: %s", result)
	}
	if result != "content here" {
		t.Fatalf("expected 'content here', got %q", result)
	}
}

func TestExecReadOutsideWorkdir(t *testing.T) {
	workdir := t.TempDir()
	w := &Worker{workdir: workdir}

	input := map[string]interface{}{
		"file_path": "/etc/hostname",
	}

	_, isErr := w.execRead(input)
	if !isErr {
		t.Fatal("expected error when reading outside workdir")
	}
}

func TestExecEdit(t *testing.T) {
	workdir := t.TempDir()
	w := &Worker{workdir: workdir}

	os.WriteFile(filepath.Join(workdir, "edit.txt"), []byte("hello world"), 0644)

	input := map[string]interface{}{
		"file_path":  "edit.txt",
		"old_string": "world",
		"new_string": "Go",
	}

	result, isErr := w.execEdit(input)
	if isErr {
		t.Fatalf("execEdit returned error: %s", result)
	}

	data, _ := os.ReadFile(filepath.Join(workdir, "edit.txt"))
	if string(data) != "hello Go" {
		t.Fatalf("expected 'hello Go', got %q", string(data))
	}
}

func TestExecBash(t *testing.T) {
	workdir := t.TempDir()
	w := &Worker{workdir: workdir}

	input := map[string]interface{}{
		"command": "echo hello-from-local",
	}

	result, isErr := w.execBash(input)
	if isErr {
		t.Fatalf("execBash returned error: %s", result)
	}
	if result != "hello-from-local\n" {
		t.Fatalf("expected 'hello-from-local\\n', got %q", result)
	}
}
