package entity

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFile(t *testing.T) {
	file := &File{
		StorageID:        "5B4BAE0D-5E07-4B36-A2C2-0DF79F558F6F",
		FormatID:         "6C5CBF1E-6F18-4C47-B3D3-1EF79F558F6F",
		FileSetID:        "7D6DCF2F-7F29-4D58-C4E4-2FG80F558F6F",
		Type:             "FILE",
		DirectoryPath:    "/test/path",
		Name:             "test.jpg",
		Size:             12345,
		FileDateCreated:  "2023-01-01T00:00:00Z",
		FileDateModified: "2023-01-01T00:00:00Z",
		AssetID:          "8E7EDF3G-8G30-5E69-D5F5-3GH91G669G7G",
	}

	fileStr, err := file.Marshal()
	assert.NoError(t, err)

	file2 := &File{}
	err = file2.Unmarshal(fileStr)
	assert.NoError(t, err)

	assert.Equal(t, file2.FormatID, "6C5CBF1E-6F18-4C47-B3D3-1EF79F558F6F")
	assert.Equal(t, file2.FileSetID, "7D6DCF2F-7F29-4D58-C4E4-2FG80F558F6F")
	assert.Equal(t, file2.Type, "FILE")
	assert.Equal(t, file2.DirectoryPath, "/test/path")
	assert.Equal(t, file2.Name, "test.jpg")
	assert.Equal(t, file2.Size, 12345)
	assert.Equal(t, file2.FileDateCreated, "2023-01-01T00:00:00Z")
	assert.Equal(t, file2.FileDateModified, "2023-01-01T00:00:00Z")
	assert.Equal(t, file2.AssetID, "8E7EDF3G-8G30-5E69-D5F5-3GH91G669G7G")
}

func TestUploadFile(t *testing.T) {
	// generate json string for upload file struct, unmarshal it and check if the values are the same
	uploadFileStr := `{
		"name": "test.jpg",
		"original_name": "test.jpg",
		"directory_path": "/test/path",
		"size": 12345,
		"type": "FILE",
		"storage_id": "5B4BAE0D-5E07-4B36-A2C2-0DF79F558F6F",
		"file_set_id": "7D6DCF2F-7F29-4D58-C4E4-2FG80F558F6F",
		"format_id": "6C5CBF1E-6F18-4C47-B3D3-1EF79F558F6F",
		"upload_url": "https://test.com/upload",
		"upload_credentials": {
			"key": "test-key",
			"secret": "test-secret"
		},
		"id": "8E7EDF3G-8G30-5E69-D5F5-3GH91G669G7G",
		"file_date_created": "2023-01-01T00:00:00Z",
		"file_date_modified": "2023-01-01T00:00:00Z"
	}`

	uploadFile := &UploadFile{}
	err := json.Unmarshal([]byte(uploadFileStr), uploadFile)
	assert.NoError(t, err)

	assert.Equal(t, uploadFile.Name, "test.jpg")
	assert.Equal(t, uploadFile.DirectoryPath, "/test/path")
	assert.Equal(t, uploadFile.Size, int64(12345))
	assert.Equal(t, uploadFile.Type, "FILE")
	assert.Equal(t, uploadFile.StorageID, "5B4BAE0D-5E07-4B36-A2C2-0DF79F558F6F")
	assert.Equal(t, uploadFile.FileSetID, "7D6DCF2F-7F29-4D58-C4E4-2FG80F558F6F")
	assert.Equal(t, uploadFile.FormatID, "6C5CBF1E-6F18-4C47-B3D3-1EF79F558F6F")
	assert.Equal(t, uploadFile.UploadURL, "https://test.com/upload")
	assert.Equal(t, uploadFile.UploadCredentials["key"], "test-key")
	assert.Equal(t, uploadFile.UploadCredentials["secret"], "test-secret")
	assert.Equal(t, uploadFile.ID, "8E7EDF3G-8G30-5E69-D5F5-3GH91G669G7G")
	assert.Equal(t, uploadFile.FileDateCreated, "2023-01-01T00:00:00Z")
	assert.Equal(t, uploadFile.FileDateModified, "2023-01-01T00:00:00Z")
}
