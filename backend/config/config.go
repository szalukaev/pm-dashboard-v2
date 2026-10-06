package config

type AppConfig struct {
	Port           string
	DSN            string // PostgreSQL connection string
	SessionSecret  string
	RedmineURL     string
	RedmineAPIKey     string
	RedmineBasicLogin string
	RedmineBasicPass  string
	DataSourceType string // "redmine", etc.
}
