package client

import (
	"context"

	"github.com/kgantsov/synconik/internal/storage"
	"github.com/stretchr/testify/mock"
)

// MockClient is a mock implementation of the Iconik client
type MockClient struct {
	mock.Mock
}

// CreateAsset mocks the CreateAsset method
func (m *MockClient) CreateAsset(ctx context.Context, asset *Asset) (*Asset, error) {
	args := m.Called(ctx, asset)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*Asset), args.Error(1)
}

// CreateCollection mocks the CreateCollection method
func (m *MockClient) CreateCollection(ctx context.Context, collection *Collection) (*Collection, error) {
	args := m.Called(ctx, collection)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*Collection), args.Error(1)
}

// CreateFileSet mocks the CreateFileSet method
func (m *MockClient) CreateFileSet(ctx context.Context, id string, fileSet *FileSet) (*FileSet, error) {
	args := m.Called(ctx, id, fileSet)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*FileSet), args.Error(1)
}

// CreateFile mocks the CreateFile method
func (m *MockClient) CreateFile(ctx context.Context, asset_id string, file *File) (*File, error) {
	args := m.Called(ctx, asset_id, file)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*File), args.Error(1)
}

// TriggerTranscoding mocks the TriggerTranscoding method
func (m *MockClient) TriggerTranscoding(ctx context.Context, asset_id, file_id string) (string, error) {
	args := m.Called(ctx, asset_id, file_id)
	return args.String(0), args.Error(1)
}

// CloseFile mocks the CloseFile method
func (m *MockClient) CloseFile(ctx context.Context, id, file_id string) error {
	args := m.Called(ctx, id, file_id)
	return args.Error(0)
}

// CreateAssetFormat mocks the CreateAssetFormat method
func (m *MockClient) CreateAssetFormat(ctx context.Context, id string, format *Format) (*Format, error) {
	args := m.Called(ctx, id, format)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*Format), args.Error(1)
}

// GetStorage mocks the GetStorage method
func (m *MockClient) GetStorage(ctx context.Context, id string) (*Storage, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*Storage), args.Error(1)
}

// Upload mocks the Upload method
func (m *MockClient) Upload(ctx context.Context, storage storage.Storage, filePath string, file *File) error {
	args := m.Called(ctx, storage, filePath, file)
	return args.Error(0)
}

// NewMockClient creates a new instance of MockClient
func NewMockClient() *MockClient {
	return &MockClient{}
}
