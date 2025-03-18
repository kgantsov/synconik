package storage

import (
	icnk_client "github.com/kgantsov/synconik/internal/iconik/client"
)

type Storage interface {
	Upload(filePath string, file *icnk_client.File) error
}
