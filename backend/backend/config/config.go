package config

type AppConfig struct {
	Port           string
	DSN            string // PostgreSQL connection string
	SessionSecret  string
	RedmineURL     string
	RedmineAPIKey  string
	DataSourceType string // "redmine", etc.
}
