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

// SearchCollections mocks the SearchCollections method
func (m *MockClient) SearchCollections(ctx context.Context, parentID, query string) ([]Collection, error) {
	args := m.Called(ctx, parentID, query)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]Collection), args.Error(1)
}

// CreateFileSet mocks the CreateFileSet method
func (m *MockClient) CreateFileSet(ctx context.Context, id string, fileSet *FileSet) (*FileSet, error) {
	args := m.Called(ctx, id, fileSet)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*FileSet), args.Error(1)
}

// DeleteFileSet mocks the DeleteFileSet method
func (m *MockClient) DeleteFileSet(ctx context.Context, asset_id, file_set_id string) error {
	args := m.Called(ctx, asset_id, file_set_id)
	return args.Error(0)
}

// GetStorageDeletions mocks the GetStorageDeletions method
func (m *MockClient) GetStorageDeletions(ctx context.Context, storage_id string) ([]FileDeletion, error) {
	args := m.Called(ctx, storage_id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]FileDeletion), args.Error(1)
}

// DeleteStorageDeletion mocks the DeleteStorageDeletion method
func (m *MockClient) DeleteStorageDeletion(ctx context.Context, storage_id, deletion_id string) error {
	args := m.Called(ctx, storage_id, deletion_id)
	return args.Error(0)
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

// GetAssetFiles mocks the GetAssetFiles method
func (m *MockClient) GetAssetFiles(ctx context.Context, asset_id string, generateSignedURL bool) ([]File, error) {
	args := m.Called(ctx, asset_id, generateSignedURL)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]File), args.Error(1)
}

// GetStorageTransfersTo mocks the GetStorageTransfersTo method
func (m *MockClient) GetStorageTransfersTo(ctx context.Context, storage_id string) ([]Transfer, error) {
	args := m.Called(ctx, storage_id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]Transfer), args.Error(1)
}

// AckStorageTransferTo mocks the AckStorageTransferTo method
func (m *MockClient) AckStorageTransferTo(ctx context.Context, storage_id, transfer_id string, success bool) error {
	args := m.Called(ctx, storage_id, transfer_id, success)
	return args.Error(0)
}

// GetStorageFiles mocks the GetStorageFiles method
func (m *MockClient) GetStorageFiles(
	ctx context.Context, storageID, directoryPath string,
) ([]File, error) {
	args := m.Called(ctx, storageID, directoryPath)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]File), args.Error(1)
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
