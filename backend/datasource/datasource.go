package datasource

import (
	"context"
	"database/sql"
)

type SourceConfig struct {
	URL    string
	APIKey string
}

type Project struct {
	ExternalID int
	Name       string
	ParentID   *int
}

type Issue struct {
	ExternalID     int
	ProjectID      int
	ProjectName    string
	Subject        string
	Description    string
	StatusName     string
	StatusID       int
	PriorityName   string
	PriorityID     int
	AssignedToName string
	AssignedToID   *int
	CategoryName   string
	StartDate      *string
	DueDate        *string
	EstimatedHours *float64
	SpentHours     float64
	DoneRatio      int
	TrackerName    string
	AuthorName     string
	// Когда задача создана, изменена в последний раз и закрыта в источнике
	// (RFC 3339); nil — источник этого не сообщает.
	CreatedOn *string
	UpdatedOn *string
	ClosedOn  *string
	// FixedVersionName: версия (спринт), на которую запланирована задача.
	FixedVersionName string
}

type Member struct {
	ExternalID int
	Name       string
	Login      string
}

type Status struct {
	ExternalID int
	Name       string
	IsClosed   bool
	GroupName  string // "open", "testing", "closed"
}

type Priority struct {
	ExternalID int
	Name       string
	SortOrder  int
	Color      string
}

type IssueFilter struct {
	ProjectID  int
	StatusName string
	Search     string
}

// DataSource — контракт, который реализует каждый источник данных.
type DataSource interface {
	Name() string
	TestConnection(ctx context.Context) error
	Sync(ctx context.Context, db *sql.DB) error
}
