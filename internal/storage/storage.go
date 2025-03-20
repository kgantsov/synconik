package storage

import (
	"github.com/kgantsov/synconik/internal/entity"
)

type Storage interface {
	Upload(filePath string, file *entity.UploadFile) error
}
