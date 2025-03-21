package store

import (
	"os"
	"testing"

	"github.com/kgantsov/synconik/internal/entity"
	"github.com/stretchr/testify/assert"
)

func setupTestDB(t *testing.T) (*BadgerStore, string, func()) {
	// Create a temporary directory for the test database
	tmpDir, err := os.MkdirTemp("", "badger-test-*")
	assert.NoError(t, err)

	// Create a new BadgerStore instance
	store, err := NewBadgerStore(tmpDir)
	assert.NoError(t, err)

	// Return cleanup function
	cleanup := func() {
		store.Close()
		os.RemoveAll(tmpDir)
	}

	return store, tmpDir, cleanup
}

func TestBadgerStore_FileOperations(t *testing.T) {
	store, _, cleanup := setupTestDB(t)
	defer cleanup()

	testFile := &entity.File{
		DirectoryPath: "/test/dir",
		Name:          "test.txt",
		Type:          "text/plain",
		Size:          100,
	}
	testPath := "/test/dir/test.txt"

	// Test SaveFile
	err := store.SaveFile(testPath, testFile)
	assert.NoError(t, err)

	// Test ExistsFile
	exists, err := store.ExistsFile(testPath)
	assert.NoError(t, err)
	assert.True(t, exists)

	// Test GetFile
	gotFile, err := store.GetFile(testPath)
	assert.NoError(t, err)
	assert.Equal(t, testFile.DirectoryPath, gotFile.DirectoryPath)
	assert.Equal(t, testFile.Name, gotFile.Name)
	assert.Equal(t, testFile.Type, gotFile.Type)
	assert.Equal(t, testFile.Size, gotFile.Size)

	// Test DeleteFile
	err = store.DeleteFile(testPath)
	assert.NoError(t, err)

	// Verify file is deleted
	exists, err = store.ExistsFile(testPath)
	assert.NoError(t, err)
	assert.False(t, exists)
}

func TestBadgerStore_Close(t *testing.T) {
	store, _, cleanup := setupTestDB(t)
	defer cleanup()

	err := store.Close()
	assert.NoError(t, err)

	// Verify store is closed by attempting an operation
	_, err = store.GetFile("/test.txt")
	assert.Error(t, err)
}
