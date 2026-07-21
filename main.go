package main

import (
	"fmt"
	"net/http"
	"os"
	"os/signal"
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

func main() {
	rootCmd := config.InitCobraCommand(Run)

	if err := rootCmd.Execute(); err != nil {
		log.Warn().Err(err)
	}
}
