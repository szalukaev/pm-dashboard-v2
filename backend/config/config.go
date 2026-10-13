package config

type AppConfig struct {
	Port              string
	DSN               string // строка подключения к PostgreSQL
	SessionSecret     string
	RedmineURL        string
	RedmineAPIKey     string
	RedmineBasicLogin string
	RedmineBasicPass  string
	DataSourceType    string // "redmine" и т. п.
}
