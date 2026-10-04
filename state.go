package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"sync"
)

func rootLock(m *sync.Map, root string) *sync.Mutex {
	v, _ := m.LoadOrStore(root, &sync.Mutex{})
	return v.(*sync.Mutex)
}

func replaceLocalFile(root string, parts []string, b []byte) error {
	p, err := writablePath(root, parts...)
	if err != nil {
		return err
	}
	old, readErr := os.ReadFile(p)
	if readErr == nil {
		if bytes.Equal(old, b) {
			return nil
		}
		if _, err = newFileAtomic(root, []string{"data", "backups", uniqueID() + "-" + filepath.Base(p)}, old); err != nil {
			return err
		}
	} else if !errors.Is(readErr, os.ErrNotExist) {
		return readErr
	}
	if err = os.MkdirAll(filepath.Dir(p), 0700); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(p), ".xiapan-save-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err = f.Write(b); err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	return os.Rename(f.Name(), p)
}
