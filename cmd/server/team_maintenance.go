package main

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"time"
)

func (r *teamEngines) maintain(ctx context.Context) {
	defer close(r.done)
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case at := <-ticker.C:
			r.cleanRuntime(at)
		}
	}
}
func (r *teamEngines) cleanRuntime(at time.Time) {
	r.mu.Lock()
	dirs := map[string]string{}
	for id, engine := range r.entries {
		if engine.owned {
			dirs[id] = engine.store.RuntimeDir()
		}
	}
	r.mu.Unlock()
	for id, dir := range dirs {
		err := filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if !entry.Type().IsRegular() {
				return nil
			}
			info, err := entry.Info()
			if err != nil {
				return err
			}
			if at.Sub(info.ModTime()) > time.Hour {
				return os.Remove(path)
			}
			return nil
		})
		r.mu.Lock()
		r.cleanupFault[id] = err != nil
		r.mu.Unlock()
	}
}
