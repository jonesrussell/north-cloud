package icpstore

import (
	"context"
	"fmt"
	"path/filepath"
	"sync/atomic"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/jonesrussell/north-cloud/infrastructure/icp"
	infralogger "github.com/jonesrussell/north-cloud/infrastructure/logger"
)

const defaultReloadInterval = 30 * time.Second

type Store struct {
	path           string
	reloadInterval time.Duration
	log            infralogger.Logger
	current        atomic.Pointer[icp.Seed]
}

func New(path string, reloadInterval time.Duration, log infralogger.Logger) (*Store, error) {
	if reloadInterval == 0 {
		reloadInterval = defaultReloadInterval
	}
	store := &Store{path: path, reloadInterval: reloadInterval, log: log}
	if err := store.reload(); err != nil {
		return nil, err
	}
	return store, nil
}

func (s *Store) Current() *icp.Seed {
	seed := s.current.Load()
	if seed == nil {
		return nil
	}
	copied := *seed
	copied.Segments = append([]icp.Segment(nil), seed.Segments...)
	return &copied
}

func (s *Store) Run(ctx context.Context) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		s.log.Warn("ICP fsnotify watcher unavailable; falling back to periodic reload",
			infralogger.Error(err),
		)
		s.runTicker(ctx)
		return
	}
	defer func() { _ = watcher.Close() }()

	dir := filepath.Dir(s.path)
	if err = watcher.Add(dir); err != nil {
		s.log.Warn("ICP seed directory watcher unavailable; falling back to periodic reload",
			infralogger.String("path", s.path),
			infralogger.Error(err),
		)
		s.runTicker(ctx)
		return
	}

	ticker := time.NewTicker(s.reloadInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case event := <-watcher.Events:
			if filepath.Clean(event.Name) == filepath.Clean(s.path) && event.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Rename) != 0 {
				s.reloadWithLog("fsnotify")
			}
		case err = <-watcher.Errors:
			s.log.Warn("ICP seed watcher error", infralogger.Error(err))
		case <-ticker.C:
			s.reloadWithLog("periodic")
		}
	}
}

func (s *Store) runTicker(ctx context.Context) {
	ticker := time.NewTicker(s.reloadInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.reloadWithLog("periodic")
		}
	}
}

func (s *Store) reloadWithLog(trigger string) {
	if err := s.reload(); err != nil {
		s.log.Warn("ICP seed reload failed",
			infralogger.String("trigger", trigger),
			infralogger.String("path", s.path),
			infralogger.Error(err),
		)
		return
	}
	s.log.Info("ICP seed reloaded",
		infralogger.String("trigger", trigger),
		infralogger.String("path", s.path),
	)
}

func (s *Store) reload() error {
	seed, err := icp.LoadSeed(s.path)
	if err != nil {
		return fmt.Errorf("load ICP seed %q: %w", s.path, err)
	}
	s.current.Store(seed)
	return nil
}
