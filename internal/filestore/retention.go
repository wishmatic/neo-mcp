package filestore

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"time"
)

func (c *Client) Sweep(now time.Time) (int, error) {
	if c.cfg.RetentionDays <= 0 {
		return 0, nil
	}

	cutoff := now.Add(-time.Duration(c.cfg.RetentionDays) * 24 * time.Hour)
	root := filepath.Join(c.cfg.Dir, namespace)

	deleted, err := deleteOlderThan(root, cutoff)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return deleted, err
	}

	removeEmptyDirs(root)

	return deleted, nil
}

func deleteOlderThan(root string, cutoff time.Time) (int, error) {
	deleted := 0

	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if entry.IsDir() {
			return nil
		}

		info, err := entry.Info()
		if err != nil {
			return err
		}

		if info.ModTime().After(cutoff) {
			return nil
		}

		if err := os.Remove(path); err != nil {
			return err
		}

		deleted++

		return nil
	})
	if err != nil {
		return deleted, err
	}

	return deleted, nil
}

func removeEmptyDirs(root string) {
	var dirs []string

	_ = filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}

		if entry.IsDir() && path != root {
			dirs = append(dirs, path)
		}

		return nil
	})

	sort.Slice(dirs, func(i, j int) bool { return len(dirs[i]) > len(dirs[j]) })

	for _, dir := range dirs {
		_ = os.Remove(dir)
	}
}
