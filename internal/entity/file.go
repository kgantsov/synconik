package entity

import "encoding/json"

type File struct {
	DirectoryPath    string `json:"directory_path"`
	Name             string `json:"name"`
	Type             string `json:"type,omitempty"`
	AssetID          string `json:"asset_id,omitempty"`
	StorageID        string `json:"storage_id,omitempty"`
	FormatID         string `json:"format_id,omitempty"`
	FileSetID        string `json:"file_set_id,omitempty"`
	ID               string `json:"id,omitempty"`
	Size             int    `json:"size,omitempty"`
	FileDateCreated  string `json:"file_date_created,omitempty"`
	FileDateModified string `json:"file_date_modified,omitempty"`
}

func (f *File) Marshal() ([]byte, error) {
	return json.Marshal(f)
}

func (f *File) Unmarshal(data []byte) error {
	return json.Unmarshal(data, f)
}

type UploadFile struct {
	Name              string            `json:"name"`
	OriginalName      string            `json:"original_name"`
	DirectoryPath     string            `json:"directory_path"`
	Size              int64             `json:"size"`
	Type              string            `json:"type"`
	StorageID         string            `json:"storage_id"`
	FileSetID         string            `json:"file_set_id"`
	FormatID          string            `json:"format_id"`
	UploadURL         string            `json:"upload_url"`
	UploadCredentials map[string]string `json:"upload_credentials"`
	ID                string            `json:"id"`
	FileDateCreated   string            `json:"file_date_created,omitempty"`
	FileDateModified  string            `json:"file_date_modified,omitempty"`
}
