package iqsync

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// PyToIQ makes stem.iqpython match stem.py in dir (FR-3.1 to FR-3.6, FR-4.1). It returns a log
// message describing the change, or "" when nothing changed.
func PyToIQ(dir, stem string) (string, error) {
	py, err := os.ReadFile(filepath.Join(dir, stem+".py"))
	if err != nil {
		return "", err
	}
	iqPath := filepath.Join(dir, stem+".iqpython")
	doc, err := os.ReadFile(iqPath)
	if errors.Is(err, fs.ErrNotExist) {
		out, err := NewIQPython(py)
		if err != nil {
			return "", err
		}
		if err := WriteFileAtomic(iqPath, out); err != nil {
			return "", err
		}
		return "Created " + stem + ".iqpython from template", nil
	}
	if err != nil {
		return "", err
	}
	out, err := PythonToIQ(doc, py)
	if err != nil {
		return "", fmt.Errorf("cannot update %s.iqpython, not overwritten: %w", stem, err)
	}
	if bytes.Equal(out, doc) {
		return "", nil
	}
	if err := WriteFileAtomic(iqPath, out); err != nil {
		return "", err
	}
	return "Updated " + stem + ".iqpython from " + stem + ".py", nil
}

// IQToPy makes stem.py match stem.iqpython in dir (FR-3.7 to FR-3.10, FR-4.3, FR-4.4). It returns
// a log message describing the change, or "" when nothing changed.
func IQToPy(dir, stem string) (string, error) {
	doc, err := os.ReadFile(filepath.Join(dir, stem+".iqpython"))
	if err != nil {
		return "", err
	}
	src, err := IQToPython(doc)
	if err != nil {
		return "", fmt.Errorf("cannot read %s.iqpython, %s.py untouched: %w", stem, stem, err)
	}
	pyPath := filepath.Join(dir, stem+".py")
	old, err := os.ReadFile(pyPath)
	created := errors.Is(err, fs.ErrNotExist)
	if err != nil && !created {
		return "", err
	}
	if !created && bytes.Equal(old, src) {
		return "", nil
	}
	if err := WriteFileAtomic(pyPath, src); err != nil {
		return "", err
	}
	if created {
		return "Created " + stem + ".py from " + stem + ".iqpython", nil
	}
	return "Updated " + stem + ".py from " + stem + ".iqpython", nil
}
