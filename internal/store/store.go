package store

import "github.com/kgantsov/synconic/internal/entity"

type Store interface {
	// Get returns the value for the given key
	GetFile(path string) (*entity.File, error)
	ExistsFile(path string) (bool, error)
	SaveFile(path string, file *entity.File) error
	DeleteFile(path string) error
}
