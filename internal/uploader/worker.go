package uploader

import (
	"os"
	"sync"

	"github.com/kgantsov/synconik/internal/config"
	icnk_client "github.com/kgantsov/synconik/internal/iconik/client"
	"github.com/kgantsov/synconik/internal/store"
	"github.com/kgantsov/synconik/internal/usecase"
	"github.com/rs/zerolog/log"
)

type Payload struct {
	Path string
	Info os.FileInfo
	WG   *sync.WaitGroup
}

type Job struct {
	Payload Payload
}

type Worker struct {
	Config *config.Config

	Name       string
	WorkerPool chan chan Job
	JobChannel chan Job
	storage    *icnk_client.Storage

	quit chan bool

	store  store.Store
	client icnk_client.Client

	assetUseCase *usecase.AssetUseCase
}

func NewWorker(
	config *config.Config,
	store store.Store,
	client icnk_client.Client,
	name string,
	workerPool chan chan Job,
	storage *icnk_client.Storage,
) *Worker {
	return &Worker{
		Config:     config,
		Name:       name,
		WorkerPool: workerPool,
		JobChannel: make(chan Job),
		storage:    storage,
		quit:       make(chan bool),

		store:  store,
		client: client,

		assetUseCase: usecase.NewAssetUseCase(config, client, store, storage),
	}
}

// Start method starts the run loop for the worker, listening for a quit channel in
// case we need to stop it
func (w *Worker) Start() {
	log.Info().Str("service", "uploader").Str("worker", w.Name).Msg("Starting worker")

	go func() {
		for {
			// register the current worker into the worker queue.
			w.WorkerPool <- w.JobChannel

			select {
			case job := <-w.JobChannel:
				log.Info().
					Str("service", "uploader").
					Str("worker", w.Name).
					Str("path", job.Payload.Path).
					Msgf("Got a job")

				err := w.assetUseCase.UploadIfNotExists(job.Payload.Path, job.Payload.Info)
				if err != nil {
					log.Error().
						Err(err).
						Str("service", "uploader").
						Str("worker", w.Name).
						Str("path", job.Payload.Path).
						Msg(
							"Error creating asset",
						)
				}

				log.Info().
					Str("service", "uploader").
					Str("worker", w.Name).
					Str("path", job.Payload.Path).
					Msg("Job done")

				job.Payload.WG.Done()

			case <-w.quit:
				// we have received a signal to stop
				log.Debug().Str("service", "uploader").Str("worker", w.Name).Msg("Stopped worker")
				return
			}
		}
	}()
}

// Stop signals the worker to stop listening for work requests.
func (w *Worker) Stop() {
	log.Debug().Str("service", "uploader").Str("worker", w.Name).Msg("Stopping worker")

	close(w.quit)
}
