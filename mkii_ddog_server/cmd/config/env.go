package config

import (
	"github.com/Nokodoko/mkii_ddog_server/cmd/utils"
)

type Config struct {
	PublicHost string
	DBHost     string
	DBPort     string
	DBUser     string
	DBPassword string
	DBName     string
	SSLMode    string

	// APM Configuration
	DDService   string
	DDEnv       string
	DDVersion   string
	DDAgentHost string
}

var Envs = initConfig()

func initConfig() Config {
	return Config{
		PublicHost: utils.GetEnv("PUBLIC_HOST", "http://localhost"),
		DBHost:     utils.GetEnv("DB_HOST", "localhost"),
		DBPort:     utils.GetEnv("DB_PORT", "5432"),
		DBUser:     utils.GetEnv("DB_USER", "rayne"),
		DBPassword: utils.GetEnv("DB_PASSWORD", "raynepassword"),
		DBName:     utils.GetEnv("DB_NAME", "rayne"),
		SSLMode:    utils.GetEnv("DB_SSLMODE", "disable"),

		// APM Configuration
		DDService:   utils.GetEnv("DD_SERVICE", "rayne"),
		DDEnv:       utils.GetEnv("DD_ENV", "development"),
		DDVersion:   utils.GetEnv("DD_VERSION", "1.0.0"),
		DDAgentHost: utils.GetEnv("DD_AGENT_HOST", "localhost"),
	}
}
