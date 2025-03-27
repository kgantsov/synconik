package config

import (
	"fmt"
	"os"
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
}

type LoggingConfig struct {
	LogLevel string `mapstructure:"level"`
}

type UploaderConfig struct {
	Workers int `mapstructure:"workers"`
}

type Iconik struct {
	URL       string `mapstructure:"url"`
	AppID     string `mapstructure:"app_id"`
	Token     string `mapstructure:"token"`
	StorageID string `mapstructure:"storage_id"`
}

type Store struct {
	DataDir string `mapstructure:"data_dir"`
}

type Config struct {
	Scanner  ScannerConfig
	Logging  LoggingConfig
	Uploader UploaderConfig
	Iconik   Iconik
	Store    Store
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

func InitCobraCommand(runFunc func(cmd *cobra.Command, args []string)) *cobra.Command {
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
		// log.Warn().Msgf("Using config file: %s", viper.ConfigFileUsed())
	}

	var rootCmd = &cobra.Command{
		Use:   "synconic",
		Short: "synconic",
		PreRunE: func(cmd *cobra.Command, args []string) error {
			requiredParams := []string{
				"scanner.dir",
				"iconik.app_id",
				"iconik.token",
				"iconik.storage_id",
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

	rootCmd.Flags().Int("uploader.workers", 5, "Number of workers to upload files")

	rootCmd.Flags().String("iconik.url", "https://app.iconik.io", "Iconik URL")
	rootCmd.Flags().String("iconik.app_id", "", "Iconik app ID")
	rootCmd.Flags().String("iconik.token", "", "Iconik token")
	rootCmd.Flags().String("iconik.storage_id", "", "Iconik storage ID")

	rootCmd.Flags().String("store.data_dir", "db", "Data directory")

	// Bind CLI flags to Viper settings
	viper.BindPFlag("logging.level", rootCmd.Flags().Lookup("logging.level"))

	viper.BindPFlag("scanner.dir", rootCmd.Flags().Lookup("scanner.dir"))
	viper.BindPFlag("scanner.interval", rootCmd.Flags().Lookup("scanner.interval"))

	viper.BindPFlag("uploader.workers", rootCmd.Flags().Lookup("uploader.workers"))

	viper.BindPFlag("iconik.url", rootCmd.Flags().Lookup("iconik.url"))
	viper.BindPFlag("iconik.app_id", rootCmd.Flags().Lookup("iconik.app_id"))
	viper.BindPFlag("iconik.token", rootCmd.Flags().Lookup("iconik.token"))
	viper.BindPFlag("iconik.storage_id", rootCmd.Flags().Lookup("iconik.storage_id"))

	viper.BindPFlag("store.data_dir", rootCmd.Flags().Lookup("store.data_dir"))

	return rootCmd
}

func (config *Config) ConfigureLogger() {
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
