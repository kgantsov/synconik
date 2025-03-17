package scanner

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/kgantsov/synconic/internal/config"
	icnk_client "github.com/kgantsov/synconic/internal/iconik/client"
	"github.com/kgantsov/synconic/internal/store"
	"github.com/kgantsov/synconic/internal/uploader"
	"github.com/kgantsov/synconic/internal/usecase"
	"github.com/rs/zerolog/log"
)

type Scanner struct {
	config         *config.Config
	UploadJobQueue chan uploader.Job

	store             store.Store
	client            *icnk_client.Client
	collectionUseCase *usecase.CollectionUseCase

	wg   *sync.WaitGroup
	done chan bool
}

func NewScanner(
	config *config.Config,
	store store.Store,
	client *icnk_client.Client,
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

		UploadJobQueue: uploadJobQueue,

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

	err := filepath.Walk(s.config.Scanner.Dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relativePath := strings.TrimPrefix(path, s.config.Scanner.Dir)

		log.Info().Str("service", "scanner").Str("path", path).Msg("Found a file or directory")

		if info.IsDir() {
			dirCount++

			log.Info().Str("service", "scanner").Str("path", path).Msgf("Found a directory")

			err := s.collectionUseCase.CreateCollectionIfNotExists(relativePath, info)
			if err != nil {
				log.Error().Err(err).Str("service", "scanner").Msgf("Error creating collection")
			}
		} else {
			fileCount++
			log.Info().Str("service", "scanner").Str("path", relativePath).Msgf("Found a file")

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

	if err != nil {
		log.Info().
			Str("service", "scanner").
			Err(err).
			Msgf("Error walking the path %q", s.config.Scanner.Dir)
	}

	log.Info().Str("service", "scanner").Msgf("Number of files in the folder: %d", fileCount)
	log.Info().Str("service", "scanner").Msgf("Number of directories in the folder: %d", dirCount)
}
