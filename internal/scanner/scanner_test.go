package scanner

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kgantsov/synconik/internal/config"
	"github.com/kgantsov/synconik/internal/iconik/client"
	"github.com/kgantsov/synconik/internal/store"
	"github.com/kgantsov/synconik/internal/uploader"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func setupTestScanner(t *testing.T) (*Scanner, string, func()) {
	// Create a temporary directory for test files
	tmpDir, err := os.MkdirTemp("", "scanner-test-*")
	assert.NoError(t, err)
	tmpDbDir, err := os.MkdirTemp("", "scanner-test-db-*")
	assert.NoError(t, err)

	testDir := filepath.Join(tmpDir, "testdir")
	err = os.MkdirAll(testDir, 0755)
	assert.NoError(t, err)

	// Create test config
	cfg := &config.Config{
		Scanner: config.ScannerConfig{
			Dir:      testDir,
			Interval: 10,
		},
	}

	store, err := store.NewBadgerStore(tmpDbDir)
	assert.NoError(t, err)

	// Create test scanner
	uploadQueue := make(chan uploader.Job, 100)
	mockClient := client.NewMockClient()

	mockClient.On(
		"CreateCollection",
		mock.Anything,
		&client.Collection{ID: "", Title: "testdir", ParentID: "", StorageID: ""},
	).Return(&client.Collection{ID: "5B4BAE0D-5E07-4B36-A2C2-0DF79F558F6F", Title: "testdir", ParentID: "", StorageID: ""}, nil)

	scanner, err := NewScanner(cfg, store, mockClient, uploadQueue)
	assert.NoError(t, err)

	cleanup := func() {
		scanner.Stop()
		os.RemoveAll(tmpDir)
		os.RemoveAll(tmpDbDir)
	}

	return scanner, tmpDir, cleanup
}

func TestScanner_StartStop(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "scanner-test-*")
	assert.NoError(t, err)
	tmpDbDir, err := os.MkdirTemp("", "scanner-test-db-*")
	assert.NoError(t, err)

	testDir := filepath.Join(tmpDir, "testdir")
	err = os.MkdirAll(testDir, 0755)
	assert.NoError(t, err)

	cfg := &config.Config{
		Scanner: config.ScannerConfig{
			Dir:      testDir,
			Interval: 10,
		},
	}

	store, err := store.NewBadgerStore(tmpDbDir)
	assert.NoError(t, err)

	// Create test scanner
	uploadQueue := make(chan uploader.Job, 100)
	mockClient := client.NewMockClient()

	mockClient.On(
		"CreateCollection",
		mock.Anything,
		&client.Collection{ID: "", Title: "testdir", ParentID: "", StorageID: ""},
	).Return(&client.Collection{ID: "5B4BAE0D-5E07-4B36-A2C2-0DF79F558F6F", Title: "testdir", ParentID: "", StorageID: ""}, nil)

	scanner, err := NewScanner(cfg, store, mockClient, uploadQueue)
	assert.NoError(t, err)

	// Test Start
	scanner.Start()
	assert.NotNil(t, scanner.done)

	// Test Stop
	scanner.Stop()
	_, ok := <-scanner.done
	assert.False(t, ok, "done channel should be closed")
}
