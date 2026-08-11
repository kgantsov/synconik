package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var cfgFile string

type ScannerConfig struct {
	Dir      string `mapstructure:"dir"`
	Interval int32  `mapstructure:"interval"`
	// StabilityWindow is the number of seconds a file's size and mtime must stay
	// unchanged before it is considered fully written and eligible for upload.
	// This prevents uploading files that are still being copied (e.g. from an SD
	// card). A value <= 0 disables the check and enqueues files immediately.
	StabilityWindow int32 `mapstructure:"stability_window"`
	// ScanIgnore is a list of glob patterns (filepath.Match syntax, e.g. "*.tmp",
	// "*.DS_Store", "cache/*"). A file or directory is skipped when a pattern
	// matches either its base name or its path relative to scanner.dir. Ignored
	// directories skip their whole subtree.
	ScanIgnore []string `mapstructure:"scan_ignore"`
}

type LoggingConfig struct {
	LogLevel string `mapstructure:"level"`
}

type UploaderConfig struct {
	Workers int `mapstructure:"workers"`
}

type Iconik struct {
	URL            string `mapstructure:"url"`
	AppID          string `mapstructure:"app_id"`
	Token          string `mapstructure:"token"`
	StorageID      string `mapstructure:"storage_id"`
	LocalStorageID string `mapstructure:"local_storage_id"`
	// CollectionID is an optional root collection. When set, top-level directories
	// (collections) and top-level files (assets) are nested under it instead of the
	// Iconik root. Nested items still resolve their parent from the local store.
	CollectionID string `mapstructure:"collection_id"`
}

type Store struct {
	DataDir string `mapstructure:"data_dir"`
}

// RestoreConfig controls ownership/permissions of files written to scanner.dir by
// the restore path (Iconik "transfer to local storage"). The daemon commonly runs
// as root, so without this the downloaded originals land as root-owned 0600 and are
// unreadable by the desktop user. Owner/Group may be a name or a numeric id; empty
// leaves that id unchanged. FileMode/DirMode are octal strings (e.g. "0644"); empty
// falls back to 0644/0755. When all four are empty the fixup is skipped entirely.
type RestoreConfig struct {
	Owner    string `mapstructure:"owner"`
	Group    string `mapstructure:"group"`
	FileMode string `mapstructure:"file_mode"`
	DirMode  string `mapstructure:"dir_mode"`
}

type Config struct {
	Scanner  ScannerConfig
	Logging  LoggingConfig
	Uploader UploaderConfig
	Iconik   Iconik
	Store    Store
	Restore  RestoreConfig
}

func LoadConfig() (*Config, error) {
	var config Config

	// Unmarshal the config into the struct
	if err := viper.Unmarshal(&config); err != nil {
		return nil, fmt.Errorf("unable to decode into struct, %v", err)
	}

	// make sure that config.Scanner.Dir has a trailing slash
	if config.Scanner.Dir[len(config.Scanner.Dir)-1] != '/' {
		config.Scanner.Dir = config.Scanner.Dir + "/"
	}

	return &config, nil
}

// StoragePrefix is the cloud-storage directory prefix, derived from the last path
// segment of scanner.dir (e.g. ".../originals/" -> "originals"). It is prepended to
// the cloud (B2/S3/GCS) directory_path so uploaded objects mirror the local folder
// name. The local ("FILE" method) storage is mounted at scanner.dir, so its paths
// stay relative and must not include this prefix.
func (c *Config) StoragePrefix() string {
	return filepath.Base(strings.TrimRight(c.Scanner.Dir, "/"))
}

func InitCobraCommand(runFunc func(cmd *cobra.Command, args []string)) *cobra.Command {
	var rootCmd = &cobra.Command{
		Use:   "synconik",
		Short: "synconik",
		// PersistentPreRunE runs after cobra has parsed the flags, so cfgFile
		// (bound to --config) is populated by now. Reading the config here — rather
		// than during command construction — is what makes --config actually work;
		// otherwise cfgFile is still empty and viper only ever finds ./config.yaml.
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			if cfgFile != "" {
				// Use config file from the flag
				viper.SetConfigFile(cfgFile)
			} else {
				// Default config file
				viper.AddConfigPath(".")
				viper.SetConfigName("config")
			}

			// Enable environment variable support
			viper.AutomaticEnv()

			// Read the config file if found
			if err := viper.ReadInConfig(); err == nil {
				log.Info().Msgf("Using config file: %s", viper.ConfigFileUsed())
			}

			return nil
		},
		PreRunE: func(cmd *cobra.Command, args []string) error {
			requiredParams := []string{
				"scanner.dir",
				"iconik.app_id",
				"iconik.token",
				"iconik.storage_id",
				"iconik.local_storage_id",
				"store.data_dir",
			}

			for _, param := range requiredParams {
				if !viper.IsSet(param) || viper.GetString(param) == "" {
					return fmt.Errorf("missing required parameter: %s", param)
				}
			}

			return nil
		},
		Run: runFunc,
	}

	// Command-line flags
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is config.yaml)")
	rootCmd.Flags().String("logging.level", "info", "Log level")

	rootCmd.Flags().String("scanner.dir", "", "Directory to scan for files")
	rootCmd.Flags().Int32("scanner.interval", 10, "Interval in seconds to scan the directory")
	rootCmd.Flags().Int32(
		"scanner.stability_window",
		30,
		"Seconds a file's size and mtime must stay unchanged before it is uploaded (0 disables the check)",
	)

	rootCmd.Flags().Int("uploader.workers", 5, "Number of workers to upload files")

	rootCmd.Flags().String("iconik.url", "https://app.iconik.io", "Iconik URL")
	rootCmd.Flags().String("iconik.app_id", "", "Iconik app ID")
	rootCmd.Flags().String("iconik.token", "", "Iconik token")
	rootCmd.Flags().String("iconik.storage_id", "", "Iconik storage ID")
	rootCmd.Flags().String("iconik.local_storage_id", "", "Iconik local (FILE method) storage ID")
	rootCmd.Flags().String(
		"iconik.collection_id", "", "Optional root Iconik collection ID to nest everything under",
	)

	rootCmd.Flags().String("store.data_dir", "db", "Data directory")

	rootCmd.Flags().String(
		"restore.owner", "", "Owner (name or uid) for files restored to scanner.dir (empty: unchanged)",
	)
	rootCmd.Flags().String(
		"restore.group", "", "Group (name or gid) for files restored to scanner.dir (empty: unchanged)",
	)
	rootCmd.Flags().String("restore.file_mode", "", "Octal mode for restored files (default 0644)")
	rootCmd.Flags().String("restore.dir_mode", "", "Octal mode for restored directories (default 0755)")

	// Bind CLI flags to Viper settings
	viper.BindPFlag("logging.level", rootCmd.Flags().Lookup("logging.level"))

	viper.BindPFlag("scanner.dir", rootCmd.Flags().Lookup("scanner.dir"))
	viper.BindPFlag("scanner.interval", rootCmd.Flags().Lookup("scanner.interval"))
	viper.BindPFlag("scanner.stability_window", rootCmd.Flags().Lookup("scanner.stability_window"))

	viper.BindPFlag("uploader.workers", rootCmd.Flags().Lookup("uploader.workers"))

	viper.BindPFlag("iconik.url", rootCmd.Flags().Lookup("iconik.url"))
	viper.BindPFlag("iconik.app_id", rootCmd.Flags().Lookup("iconik.app_id"))
	viper.BindPFlag("iconik.token", rootCmd.Flags().Lookup("iconik.token"))
	viper.BindPFlag("iconik.storage_id", rootCmd.Flags().Lookup("iconik.storage_id"))
	viper.BindPFlag("iconik.local_storage_id", rootCmd.Flags().Lookup("iconik.local_storage_id"))
	viper.BindPFlag("iconik.collection_id", rootCmd.Flags().Lookup("iconik.collection_id"))

	viper.BindPFlag("store.data_dir", rootCmd.Flags().Lookup("store.data_dir"))

	viper.BindPFlag("restore.owner", rootCmd.Flags().Lookup("restore.owner"))
	viper.BindPFlag("restore.group", rootCmd.Flags().Lookup("restore.group"))
	viper.BindPFlag("restore.file_mode", rootCmd.Flags().Lookup("restore.file_mode"))
	viper.BindPFlag("restore.dir_mode", rootCmd.Flags().Lookup("restore.dir_mode"))

	return rootCmd
}

func (config *Config) ConfigureLogger() {
	zerolog.CallerMarshalFunc = func(pc uintptr, file string, line int) string {
		return filepath.Join(filepath.Base(filepath.Dir(file)), filepath.Base(file)) +
			":" + strconv.Itoa(line)
	}
	log.Logger = log.Output(
		zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339Nano},
	).With().Caller().Logger()
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnixNano

	logLevel, err := zerolog.ParseLevel(config.Logging.LogLevel)
	if err != nil {
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	} else {
		zerolog.SetGlobalLevel(logLevel)
	}
}
