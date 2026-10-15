package repository

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	models "github.com/kanataidarov/kanataidarov-go-advanced-spr1/internal/model"
)

func ptrFloat(v float64) *float64 { return &v }
func ptrInt(v int64) *int64       { return &v }

func TestFileStoreSaveLoad(t *testing.T) {
	path := filepath.Join(t.TempDir(), "metrics.json")
	store := NewFileStore(path)

	saved := []models.Metrics{
		{ID: "NumGC", MType: models.Counter, Delta: ptrInt(42)},
		{ID: "LastGC", MType: models.Gauge, Value: ptrFloat(1257894000000000000)},
		{ID: "Alloc", MType: models.Gauge, Value: ptrFloat(12.5)},
	}

	if err := store.Save(saved); err != nil {
		t.Fatalf("save: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	want := "[\n" +
		`{"id":"NumGC","type":"counter","delta":42},` + "\n" +
		`{"id":"Alloc","type":"gauge","value":12.5},` + "\n" +
		`{"id":"LastGC","type":"gauge","value":1.257894e+18}` + "\n]\n"
	if string(data) != want {
		t.Errorf("got file:\n%s\nwant:\n%s", data, want)
	}

	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	wantLoaded := []models.Metrics{saved[0], saved[2], saved[1]}
	if !reflect.DeepEqual(loaded, wantLoaded) {
		t.Errorf("got %+v, want %+v", loaded, wantLoaded)
	}
}

func TestFileStoreSaveOverwritesWithoutLeftovers(t *testing.T) {
	dir := t.TempDir()
	store := NewFileStore(filepath.Join(dir, "metrics.json"))

	if err := store.Save([]models.Metrics{{ID: "A", MType: models.Gauge, Value: ptrFloat(1)}}); err != nil {
		t.Fatal(err)
	}

	if err := store.Save(nil); err != nil {
		t.Fatal(err)
	}

	loaded, err := store.Load()
	if err != nil {
		t.Fatal(err)
	}

	if len(loaded) != 0 {
		t.Errorf("got %+v, want no metrics after overwrite", loaded)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}

	if len(entries) != 1 {
		t.Errorf("got %d files in the directory, want only metrics.json", len(entries))
	}
}

func TestFileStoreLoadWithoutData(t *testing.T) {
	dir := t.TempDir()

	empty := filepath.Join(dir, "empty.json")
	if err := os.WriteFile(empty, []byte(" \n"), 0o600); err != nil {
		t.Fatal(err)
	}

	for _, path := range []string{filepath.Join(dir, "missing.json"), empty} {
		loaded, err := NewFileStore(path).Load()
		if err != nil {
			t.Errorf("%s: unexpected error: %v", filepath.Base(path), err)
		}

		if loaded != nil {
			t.Errorf("%s: got %+v, want nil", filepath.Base(path), loaded)
		}
	}
}

func TestFileStoreLoadRejectsCorruptFile(t *testing.T) {
	cases := []string{
		`not json`,
		`[{"id":"A","type":"gauge","value":1}`,
		`{"id":"A","type":"gauge","value":1}`,
		`[{"id":"A","type":"gauge","value":"x"}]`,
		`[] trailing`,
	}

	for _, data := range cases {
		path := filepath.Join(t.TempDir(), "metrics.json")
		if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
			t.Fatal(err)
		}

		if _, err := NewFileStore(path).Load(); err == nil || !strings.Contains(err.Error(), path) {
			t.Errorf("%q: got error %v, want a parse error mentioning the file", data, err)
		}
	}
}

func TestFileStoreSaveFailsForMissingDirectory(t *testing.T) {
	store := NewFileStore(filepath.Join(t.TempDir(), "no-such-dir", "metrics.json"))

	if err := store.Save(nil); err == nil {
		t.Error("expected an error")
	}
}
