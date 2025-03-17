package storage

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"io"
	"os"
)

// ComputeSHA1 calculates the SHA-1 hash of a given file
func ComputeSHA1(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	hasher := sha1.New()
	if _, err := io.Copy(hasher, file); err != nil {
		return "", fmt.Errorf("failed to compute SHA-1: %w", err)
	}

	sha1Sum := hasher.Sum(nil)
	return hex.EncodeToString(sha1Sum), nil
}
