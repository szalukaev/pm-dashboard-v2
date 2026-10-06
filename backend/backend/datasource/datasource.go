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

// DataSource is the contract every data source must implement.
type DataSource interface {
	Name() string
	TestConnection(ctx context.Context) error
	Sync(ctx context.Context, db *sql.DB) error
}
