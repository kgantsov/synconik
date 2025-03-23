package storage

import (
	"github.com/kgantsov/synconik/internal/entity"
	"github.com/stretchr/testify/mock"
)

type MockStorage struct {
	mock.Mock
}

func (s *MockStorage) Upload(filePath string, file *entity.UploadFile) error {
	args := s.Called(filePath, file)

	return args.Error(0)
}

func NewMockStorage() *MockStorage {
	return &MockStorage{}
}
