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
	}
}
