package uploader

import (
	"context"
	"fmt"

	"github.com/kgantsov/synconic/internal/config"
	icnk_client "github.com/kgantsov/synconic/internal/iconik/client"
	"github.com/kgantsov/synconic/internal/store"
	"github.com/rs/zerolog/log"
)

type Uploader struct {
	Config     *config.Config
	WorkerPool chan chan Job
	MaxWorkers int
	JobQueue   chan Job
	quit       chan bool
	Workers    []*Worker

	store  store.Store
	client *icnk_client.Client
}

func NewUploader(
	config *config.Config, store store.Store, client *icnk_client.Client, JobQueue chan Job,
) *Uploader {
	numberOfWorkers := config.Uploader.Workers

	if numberOfWorkers <= 0 {
		numberOfWorkers = 1
	}
	WorkerPool := make(chan chan Job, numberOfWorkers)

	return &Uploader{
		Config:     config,
		WorkerPool: WorkerPool,
		MaxWorkers: numberOfWorkers,
		JobQueue:   JobQueue,
		quit:       make(chan bool),
		Workers:    []*Worker{},

		store:  store,
		client: client,
	}
}

func (u *Uploader) Start() error {
	ctx := context.Background()
	storage, err := u.client.GetStorage(ctx, u.Config.Iconik.StorageID)
	if err != nil {
		log.Error().Err(err).Str("service", "uploader").Msgf("Error getting storage")
		return err
	}

	// Start the uploader
	for i := 0; i < u.MaxWorkers; i++ {
		worker := NewWorker(
			u.Config,
			u.store,
			u.client,
			fmt.Sprintf("worker-%d", i),
			u.WorkerPool,
			storage,
		)
		worker.Start()

		u.Workers = append(u.Workers, worker)
	}

	go u.dispatch()

	return nil
}

func (u *Uploader) dispatch() {
	for {
		select {
		case job := <-u.JobQueue:
			// a job request has been received
			go func(job Job) {
				// try to obtain a worker job channel that is available.
				// this will block until a worker is idle
				jobChannel := <-u.WorkerPool

				// dispatch the job to the worker job channel
				jobChannel <- job
			}(job)
		}
	}
}

func (u *Uploader) Stop() {
	for _, worker := range u.Workers {
		worker.Stop()
	}
	log.Debug().Str("service", "uploader").Msg("Stopped uploader")
	close(u.quit)
}
