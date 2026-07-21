package storage

// func TestGCSStorage_Upload(t *testing.T) {
// 	// Create a temp file with test content
// 	tmpDir, err := os.MkdirTemp("", "gcsstorage-test-*")
// 	assert.NoError(t, err)
// 	defer os.RemoveAll(tmpDir)

// 	testFile := filepath.Join(tmpDir, "test.txt")
// 	testContent := []byte("test content")
// 	err = os.WriteFile(testFile, testContent, 0644)
// 	assert.NoError(t, err)

// 	// Create mock GCS server
// 	var uploadID string
// 	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
// 		if r.Method == "POST" {
// 			// Handle initial upload request
// 			assert.Equal(t, "start", r.Header.Get("x-goog-resumable"))
// 			uploadID = "test-upload-id"
// 			w.Header().Set("X-GUploader-UploadID", uploadID)
// 			w.WriteHeader(http.StatusCreated)
// 			return
// 		}

// 		// Handle actual upload request
// 		assert.Equal(t, "PUT", r.Method)
// 		assert.Equal(t, "start", r.Header.Get("x-goog-resumable"))
// 		assert.Contains(t, r.URL.String(), "upload_id="+uploadID)

// 		// Read and verify uploaded content
// 		body, err := io.ReadAll(r.Body)
// 		assert.NoError(t, err)
// 		assert.Equal(t, testContent, body)

// 		w.WriteHeader(http.StatusOK)
// 	}))
// 	defer server.Close()

// 	// Create GCSStorage instance
// 	storage := NewGCSStorage(server.Client())

// 	// Test file upload
// 	uploadFile := &entity.UploadFile{
// 		Name:          "test.txt",
// 		DirectoryPath: "test/dir",
// 		Size:          int64(len(testContent)),
// 		UploadURL:     server.URL,
// 	}

// 	err = storage.Upload(testFile, uploadFile)
// 	assert.NoError(t, err)
// }
