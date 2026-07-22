package main

import (
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"

	"github.com/kgantsov/synconik/internal/config"
	icnk_client "github.com/kgantsov/synconik/internal/iconik/client"
	"github.com/kgantsov/synconik/internal/scanner"
	"github.com/kgantsov/synconik/internal/store"
	"github.com/kgantsov/synconik/internal/uploader"
	"github.com/kgantsov/synconik/internal/usecase"
)

func Run(cmd *cobra.Command, args []string) {

	config, err := config.LoadConfig()
	if err != nil {
		fmt.Printf("Error loading config: %v\n", err)
		return
	}

	config.ConfigureLogger()

	httpClient := &http.Client{
		Timeout: 30 * time.Second,
	}

	client := icnk_client.NewClient(httpClient, config.Iconik.URL, config.Iconik.AppID, config.Iconik.Token)

	badgerStore, err := store.NewBadgerStore(config.Store.DataDir)

	if err != nil {
		log.Error().Msgf("Error creating db: %v", err)
		return
	}

	var uploadJobQueue chan uploader.Job
	uploadJobQueue = make(chan uploader.Job)

	uploader := uploader.NewUploader(config, badgerStore, client, uploadJobQueue)
	err = uploader.Start()

	if err != nil {
		log.Error().Msgf("Error starting uploader: %v", err)
		return
	}

	scanner, err := scanner.NewScanner(config, badgerStore, client, uploadJobQueue)
	if err != nil {
		log.Error().Msgf("Error creating scanner: %v", err)
		return
	}
	scanner.Start()

	// Poll Iconik for transfers of file sets onto the local storage and pull the
	// originals back to disk, and (when enabled) for queued deletions to remove
	// local files whose file set was deleted in Iconik.
	restoreUseCase := usecase.NewRestoreUseCase(config, client, badgerStore)
	deletionUseCase := usecase.NewDeletionUseCase(config, client, badgerStore)
	poll := func() {
		restoreUseCase.RestoreRequested()
		deletionUseCase.ProcessDeletions()
	}
	restoreTicker := time.NewTicker(time.Duration(config.Scanner.Interval) * time.Second)
	restoreDone := make(chan struct{})
	go func() {
		poll()
		for {
			select {
			case <-restoreTicker.C:
				poll()
			case <-restoreDone:
				restoreTicker.Stop()
				return
			}
		}
	}()

	done := make(chan struct{})
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigs
		log.Info().Msgf("Got signal: %s", sig)

		done <- struct{}{}
	}()

	<-done

	close(restoreDone)
	scanner.Stop()
	uploader.Stop()

	time.Sleep(time.Second * 1)
}

// newForgetCommand builds the `forget` subcommand, which evicts files from the local
// BadgerDB sync store so the next scan reprocesses them. Iconik-side deletes/purges
// never touch the store, so a purged asset is otherwise skipped forever (its path
// still resolves via ExistsFile). After forgetting, the next scan re-maps files whose
// bytes still live on cloud storage and re-uploads the genuinely missing ones.
//
// The store is single-process locked by BadgerDB, so the daemon must be stopped first.
func newForgetCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "forget <path>...",
		Short: "Remove files from the local sync store so they are reprocessed on the next scan",
		Long: "Deletes the given files' records from the BadgerDB sync store. Paths may be " +
			"absolute or relative to scanner.dir. Stop the daemon first — the store is " +
			"single-process locked.",
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.LoadConfig()
			if err != nil {
				return fmt.Errorf("loading config: %w", err)
			}

			badgerStore, err := store.NewBadgerStore(cfg.Store.DataDir)
			if err != nil {
				return fmt.Errorf(
					"opening store at %s (is the daemon still running?): %w",
					cfg.Store.DataDir, err,
				)
			}
			defer badgerStore.Close()

			for _, arg := range args {
				// Store keys are relative to scanner.dir; accept absolute paths too.
				key := strings.TrimPrefix(arg, cfg.Scanner.Dir)

				exists, err := badgerStore.ExistsFile(key)
				if err != nil {
					return fmt.Errorf("checking %q: %w", key, err)
				}
				if !exists {
					fmt.Printf("not in store, skipping: %s\n", key)
					continue
				}
				if err := badgerStore.DeleteFile(key); err != nil {
					return fmt.Errorf("deleting %q: %w", key, err)
				}
				fmt.Printf("forgot: %s\n", key)
			}
			return nil
		},
	}
}

func main() {
	rootCmd := config.InitCobraCommand(Run)
	rootCmd.AddCommand(newForgetCommand())

	if err := rootCmd.Execute(); err != nil {
		log.Warn().Err(err)
	}
}
