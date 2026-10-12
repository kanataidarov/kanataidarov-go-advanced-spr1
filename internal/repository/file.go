package repository

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"

	"github.com/mailru/easyjson/jlexer"
	"github.com/mailru/easyjson/jwriter"

	models "github.com/kanataidarov/kanataidarov-go-advanced-spr1/internal/model"
)

type FileStore struct {
	path string
}

func NewFileStore(path string) *FileStore {
	return &FileStore{path: path}
}

func (f *FileStore) Path() string {
	return f.path
}

func (f *FileStore) Save(metrics []models.Metrics) error {
	sorted := append([]models.Metrics(nil), metrics...)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].MType != sorted[j].MType {
			return sorted[i].MType < sorted[j].MType
		}

		return sorted[i].ID < sorted[j].ID
	})

	w := jwriter.Writer{}
	w.RawByte('[')

	for i, m := range sorted {
		if i > 0 {
			w.RawByte(',')
		}

		w.RawByte('\n')
		m.MarshalEasyJSON(&w)
	}

	w.RawString("\n]\n")

	data, err := w.BuildBytes()
	if err != nil {
		return fmt.Errorf("encode metrics: %w", err)
	}

	tmp, err := os.CreateTemp(filepath.Dir(f.path), filepath.Base(f.path)+".tmp-*")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}

	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }()

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()

		return fmt.Errorf("write %s: %w", tmpName, err)
	}

	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()

		return fmt.Errorf("sync %s: %w", tmpName, err)
	}

	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close %s: %w", tmpName, err)
	}

	if err := os.Rename(tmpName, f.path); err != nil {
		return fmt.Errorf("replace %s: %w", f.path, err)
	}

	return nil
}

func (f *FileStore) Load() ([]models.Metrics, error) {
	data, err := os.ReadFile(f.path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}

	if err != nil {
		return nil, fmt.Errorf("read %s: %w", f.path, err)
	}

	if len(bytes.TrimSpace(data)) == 0 {
		return nil, nil
	}

	var metrics []models.Metrics

	in := jlexer.Lexer{Data: data}
	in.Delim('[')

	for !in.IsDelim(']') {
		var m models.Metrics
		m.UnmarshalEasyJSON(&in)
		metrics = append(metrics, m)
		in.WantComma()
	}

	in.Delim(']')
	in.Consumed()

	if err := in.Error(); err != nil {
		return nil, fmt.Errorf("parse %s: %w", f.path, err)
	}

	return metrics, nil
}
