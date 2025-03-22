package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConfig(t *testing.T) {
	config := &Config{}
	config.ConfigureLogger()

	assert.Equal(t, config.Logging.LogLevel, "")
	assert.Equal(t, config.Scanner.Dir, "")
	assert.Equal(t, config.Scanner.Interval, int32(0))
	assert.Equal(t, config.Uploader.Workers, 0)
	assert.Equal(t, config.Iconik.URL, "")
	assert.Equal(t, config.Iconik.AppID, "")
	assert.Equal(t, config.Iconik.Token, "")
	assert.Equal(t, config.Iconik.StorageID, "")
	assert.Equal(t, config.Store.DataDir, "")

	config = &Config{
		Logging: LoggingConfig{
			LogLevel: "debug",
		},
		Scanner: ScannerConfig{
			Dir:      "test_dir_for_scan",
			Interval: 10,
		},
		Uploader: UploaderConfig{
			Workers: 5,
		},
		Iconik: Iconik{
			URL:       "https://app.iconik.io/",
			AppID:     "123e4567-e89b-12d3-a456-426614174000",
			Token:     "abcdef0123456789abcdef0123456789",
			StorageID: "5B4BAE0D-5E07-4B36-A2C2-0DF79F558F6F",
		},
		Store: Store{
			DataDir: "db_dir",
		},
	}

	assert.Equal(t, config.Logging.LogLevel, "debug")
	assert.Equal(t, config.Scanner.Dir, "test_dir_for_scan")
	assert.Equal(t, config.Scanner.Interval, int32(10))
	assert.Equal(t, config.Uploader.Workers, 5)
	assert.Equal(t, config.Iconik.URL, "https://app.iconik.io/")
	assert.Equal(t, config.Iconik.AppID, "123e4567-e89b-12d3-a456-426614174000")
	assert.Equal(t, config.Iconik.Token, "abcdef0123456789abcdef0123456789")
	assert.Equal(t, config.Iconik.StorageID, "5B4BAE0D-5E07-4B36-A2C2-0DF79F558F6F")
	assert.Equal(t, config.Store.DataDir, "db_dir")
}
