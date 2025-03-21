package storage

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/kgantsov/synconik/internal/entity"
	"github.com/stretchr/testify/assert"
)

func TestB2Storage_Upload(t *testing.T) {
	// Create a temp file with test content
	tmpDir, err := os.MkdirTemp("", "b2storage-test-*")
	assert.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	testFile := filepath.Join(tmpDir, "test.txt")
	testContent := []byte("test content")
	err = os.WriteFile(testFile, testContent, 0644)
	assert.NoError(t, err)

	// Create mock B2 server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request headers
		assert.Equal(t, "POST", r.Method)
		assert.Equal(t, "test-auth-token", r.Header.Get("Authorization"))
		assert.Equal(t, "test/dir/test.txt", r.Header.Get("X-Bz-File-Name"))
		assert.Equal(t, "b2/x-auto", r.Header.Get("Content-Type"))
		assert.Equal(t, fmt.Sprintf("%d", len(testContent)), r.Header.Get("Content-Length"))

		// Read and verify uploaded content
		body, err := io.ReadAll(r.Body)
		assert.NoError(t, err)
		assert.Equal(t, testContent, body)

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Create B2Storage instance
	storage := NewB2Storage(server.Client())

	// Test file upload
	uploadFile := &entity.UploadFile{
		Name:          "test.txt",
		DirectoryPath: "test/dir",
		Size:          int64(len(testContent)),
		UploadURL:     server.URL,
		UploadCredentials: map[string]string{
			"authorizationToken": "test-auth-token",
		},
	}

	err = storage.Upload(testFile, uploadFile)
	assert.NoError(t, err)
}
