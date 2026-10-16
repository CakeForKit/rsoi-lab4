package config

import (
	"fmt"

	"github.com/spf13/viper"
)

func Load(config any) error {
	viperConfig := viper.New()
	viperConfig.AutomaticEnv()
	_ = viperConfig.BindEnv("port", "PORT")
	_ = viperConfig.BindEnv("schemaDB", "SCHEMA_DB")
	_ = viperConfig.BindEnv("postgres.dsn", "DATABASE_URL")
	viperConfig.AddConfigPath("./config")
	viperConfig.SetConfigName("application")
	viperConfig.SetConfigType("yaml")

	if err := viperConfig.ReadInConfig(); err != nil {
		return fmt.Errorf("failed to read configuration file: %s\n", err.Error())
	}
	if err := viperConfig.Unmarshal(config); err != nil {
		return fmt.Errorf("unmarshal config: %w", err)
	}
	return nil
}
