package config

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Port                   string
	DatabaseURL            string
	DatabaseMaxConns       int32
	DatabaseConnectTimeout time.Duration
}

// Load reads process environment. Local .env files must be explicitly sourced.
func Load() (Config, error) {
	c := Config{Port: "8080", DatabaseURL: strings.TrimSpace(os.Getenv("DATABASE_URL")), DatabaseMaxConns: 10, DatabaseConnectTimeout: 5 * time.Second}
	if v := os.Getenv("APP_PORT"); v != "" {
		c.Port = v
	}
	port, err := strconv.Atoi(c.Port)
	if err != nil || port < 1 || port > 65535 {
		return Config{}, fmt.Errorf("APP_PORT must be between 1 and 65535")
	}
	if c.DatabaseURL == "" {
		return Config{}, fmt.Errorf("DATABASE_URL is required")
	}
	if v := os.Getenv("DATABASE_MAX_CONNS"); v != "" {
		n, err := strconv.ParseInt(v, 10, 32)
		if err != nil || n < 1 {
			return Config{}, fmt.Errorf("DATABASE_MAX_CONNS must be a positive integer")
		}
		c.DatabaseMaxConns = int32(n)
	}
	if v := os.Getenv("DATABASE_CONNECT_TIMEOUT"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil || d <= 0 {
			return Config{}, fmt.Errorf("DATABASE_CONNECT_TIMEOUT must be a positive duration")
		}
		c.DatabaseConnectTimeout = d
	}
	return c, nil
}

func (c Config) Address() string { return net.JoinHostPort("", c.Port) }
