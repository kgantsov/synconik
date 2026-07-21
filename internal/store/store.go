package store

import "github.com/kgantsov/synconik/internal/entity"

// FileEntry pairs a stored file with its relative path key.
type FileEntry struct {
	Path string
	File *entity.File
}

type Store interface {
	// Get returns the value for the given key
	GetFile(path string) (*entity.File, error)
	ExistsFile(path string) (bool, error)
	SaveFile(path string, file *entity.File) error
	DeleteFile(path string) error
	// ListFiles returns every stored file with its relative path.
	ListFiles() ([]FileEntry, error)
}
