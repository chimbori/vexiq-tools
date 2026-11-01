package iqsync

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// Result describes a completed sync. Target is the file that was written and Data its content.
// Message is the activity log entry. A sync that changed nothing returns a zero Result.
type Result struct {
	Message string
	Target  string
	Data    []byte
}

// SyncFile syncs the pair that name belongs to in dir. name is a .py or .iqpython file name.
func SyncFile(dir, name string) (Result, error) {
	switch {
	case strings.HasSuffix(name, ".py"):
		return PyToIQ(dir, strings.TrimSuffix(name, ".py"))
	case strings.HasSuffix(name, ".iqpython"):
		return IQToPy(dir, strings.TrimSuffix(name, ".iqpython"))
	}
	return Result{}, fmt.Errorf("%s is not a .py or .iqpython file", name)
}

// PyToIQ makes stem.iqpython match stem.py in dir (FR-3.1 to FR-3.6, FR-4.1).
func PyToIQ(dir, stem string) (Result, error) {
	py, err := os.ReadFile(filepath.Join(dir, stem+".py"))
	if err != nil {
		return Result{}, err
	}
	target := stem + ".iqpython"
	iqPath := filepath.Join(dir, target)
	doc, err := os.ReadFile(iqPath)
	if errors.Is(err, fs.ErrNotExist) {
		out, err := NewIQPython(py)
		if err != nil {
			return Result{}, err
		}
		if err := WriteFileAtomic(iqPath, out); err != nil {
			return Result{}, err
		}
		return Result{"Created " + target + " from template", target, out}, nil
	}
	if err != nil {
		return Result{}, err
	}
	out, err := PythonToIQ(doc, py)
	if err != nil {
		return Result{}, fmt.Errorf("cannot update %s, not overwritten: %w", target, err)
	}
	if bytes.Equal(out, doc) {
		return Result{}, nil
	}
	if err := WriteFileAtomic(iqPath, out); err != nil {
		return Result{}, err
	}
	return Result{"Updated " + target + " from " + stem + ".py", target, out}, nil
}

// IQToPy makes stem.py match stem.iqpython in dir (FR-3.7 to FR-3.10, FR-4.3, FR-4.4).
func IQToPy(dir, stem string) (Result, error) {
	doc, err := os.ReadFile(filepath.Join(dir, stem+".iqpython"))
	if err != nil {
		return Result{}, err
	}
	src, err := IQToPython(doc)
	if err != nil {
		return Result{}, fmt.Errorf("cannot read %s.iqpython, %s.py untouched: %w", stem, stem, err)
	}
	target := stem + ".py"
	pyPath := filepath.Join(dir, target)
	old, err := os.ReadFile(pyPath)
	created := errors.Is(err, fs.ErrNotExist)
	if err != nil && !created {
		return Result{}, err
	}
	if !created && bytes.Equal(old, src) {
		return Result{}, nil
	}
	if err := WriteFileAtomic(pyPath, src); err != nil {
		return Result{}, err
	}
	if created {
		return Result{"Created " + target + " from " + stem + ".iqpython", target, src}, nil
	}
	return Result{"Updated " + target + " from " + stem + ".iqpython", target, src}, nil
}
