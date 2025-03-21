package storage

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/kgantsov/synconik/internal/entity"
	"github.com/stretchr/testify/assert"
)

func TestS3Storage_Upload(t *testing.T) {
	// Create a temp file with test content
	tmpDir, err := os.MkdirTemp("", "s3storage-test-*")
	assert.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	testFile := filepath.Join(tmpDir, "test.txt")
	testContent := []byte("test content")
	err = os.WriteFile(testFile, testContent, 0644)
	assert.NoError(t, err)

	// Create mock S3 server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request headers
		assert.Equal(t, "PUT", r.Method)
		assert.Equal(t, "application/octet-stream", r.Header.Get("Content-Type"))
		// assert.Equal(t, fmt.Sprintf("%d", len(testContent)), r.Header.Get("Content-Length"))

		// Read and verify uploaded content
		body, err := io.ReadAll(r.Body)
		assert.NoError(t, err)
		assert.Equal(t, testContent, body)

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Create S3Storage instance
	storage := NewS3Storage(server.Client())

	// Test file upload
	uploadFile := &entity.UploadFile{
		Name:          "test.txt",
		DirectoryPath: "test/dir",
		Size:          int64(len(testContent)),
		UploadURL:     server.URL,
	}

	err = storage.Upload(testFile, uploadFile)
	assert.NoError(t, err)
}
