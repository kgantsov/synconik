package scanner

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/kgantsov/synconik/internal/config"
	icnk_client "github.com/kgantsov/synconik/internal/iconik/client"
	"github.com/kgantsov/synconik/internal/store"
	"github.com/kgantsov/synconik/internal/uploader"
	"github.com/kgantsov/synconik/internal/usecase"
	"github.com/rs/zerolog/log"
)

// fileState records a file's size and mtime along with the wall-clock time we
// first observed that exact (size, mtime) pair. It is used to decide whether a
// file has stopped changing long enough to be safely uploaded.
type fileState struct {
	size      int64
	mtime     time.Time
	firstSeen time.Time
}

type Scanner struct {
	config         *config.Config
	UploadJobQueue chan uploader.Job

	store             store.Store
	client            icnk_client.Client
	collectionUseCase *usecase.CollectionUseCase
	reconcileUseCase  *usecase.ReconcileUseCase

	// stability tracks the (size, mtime, firstSeen) of files seen in the previous
	// scan so growing files can be held back until they quiesce. Only accessed
	// from Scan(), which never runs concurrently with itself.
	stability map[string]fileState

	wg   *sync.WaitGroup
	done chan bool
}

func NewScanner(
	config *config.Config,
	store store.Store,
	client icnk_client.Client,
	uploadJobQueue chan uploader.Job,
) (*Scanner, error) {
	if config.Scanner.Dir == "" {
		return nil, errors.New("scanner directory cannot be empty")
	}
	if config.Scanner.Interval <= 0 {
		return nil, errors.New("scanner interval must be positive")
	}
	var wg sync.WaitGroup

	return &Scanner{
		config: config,
		client: client,
		store:  store,

		collectionUseCase: usecase.NewCollectionUseCase(config, client, store),
		reconcileUseCase:  usecase.NewReconcileUseCase(config, client, store),

		UploadJobQueue: uploadJobQueue,

		stability: make(map[string]fileState),

		wg:   &wg,
		done: make(chan bool),
	}, nil
}

func (s *Scanner) start() {
	ticker := time.NewTicker(time.Duration(s.config.Scanner.Interval) * time.Second)
	for {
		select {
		case <-ticker.C:
			s.Scan()
		case <-s.done:
			log.Debug().Str("service", "scanner").Msg("Stopped the scanner")
			return
		}
	}
}

func (s *Scanner) Start() {
	// Start the scanner
	log.Debug().Str("service", "scanner").Msg("Starting the scanner")

	s.Scan()
	go s.start()
}

func (s *Scanner) Stop() {
	// Stop the scanner
	log.Debug().Str("service", "scanner").Msg("Stopping the scanner")

	s.wg.Wait()
	close(s.done)
}

func (s *Scanner) Scan() {
	// Scan the folder
	fileCount := 0
	dirCount := 0

	now := time.Now()
	stabilityWindow := time.Duration(s.config.Scanner.StabilityWindow) * time.Second

	// seen collects the state of every file observed in this scan and becomes the
	// baseline for the next scan. Rebuilding it each pass also prunes entries for
	// files that have since been deleted.
	seen := make(map[string]fileState)

	err := filepath.Walk(s.config.Scanner.Dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relativePath := strings.TrimPrefix(path, s.config.Scanner.Dir)

		// Skip hidden files/dirs (e.g. .DS_Store). For a hidden directory skip
		// its whole subtree; relativePath is empty only for the scan root itself,
		// which we never want to skip.
		if relativePath != "" && strings.HasPrefix(info.Name(), ".") {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		if info.IsDir() {
			dirCount++

			log.Info().
				Str("service", "scanner").
				Str("path", relativePath).
				Msgf("Found a directory %s", info.Name())

			err := s.collectionUseCase.CreateCollectionIfNotExists(relativePath, info)
			if err != nil {
				log.Error().Err(err).Str("service", "scanner").Msgf("Error creating collection")
			}
		} else {
			fileCount++
			log.Info().
				Str("service", "scanner").
				Str("path", relativePath).
				Msgf("Found a file %s", info.Name())

			// Carry forward firstSeen if the file's size and mtime are unchanged
			// since the previous scan; otherwise treat it as freshly changing.
			cur := fileState{size: info.Size(), mtime: info.ModTime(), firstSeen: now}
			if prev, ok := s.stability[relativePath]; ok &&
				prev.size == cur.size && prev.mtime.Equal(cur.mtime) {
				cur.firstSeen = prev.firstSeen
			}
			seen[relativePath] = cur

			// Hold back files that haven't been quiescent for the stability window;
			// they'll be reconsidered on the next scan once they settle.
			if stabilityWindow > 0 && now.Sub(cur.firstSeen) < stabilityWindow {
				log.Debug().
					Str("service", "scanner").
					Str("path", relativePath).
					Msgf("File %s still changing, deferring upload", info.Name())
				return nil
			}

			s.wg.Add(1)

			select {
			case s.UploadJobQueue <- uploader.Job{
				Payload: uploader.Payload{Path: relativePath, Info: info, WG: s.wg},
			}:
			case <-s.done:
				return filepath.SkipAll
			}
		}
		return nil
	})

	s.stability = seen

	if err != nil {
		log.Info().
			Str("service", "scanner").
			Err(err).
			Msgf("Error walking the path %q", s.config.Scanner.Dir)
	}

	log.Info().Str("service", "scanner").Msgf("Number of files in the folder: %d", fileCount)
	log.Info().Str("service", "scanner").Msgf("Number of directories in the folder: %d", dirCount)

	// After discovering new files, reconcile the store against disk to remove the
	// local file_set for any files that were deleted since the last scan.
	s.reconcileUseCase.ReconcileDeletions()
}
