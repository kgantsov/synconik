package storage

import (
	icnk_client "github.com/kgantsov/synconic/internal/iconik/client"
)

type Storage interface {
	Upload(filePath string, file *icnk_client.File) error
}
