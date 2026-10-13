package redmine

import "pm-dashboard/datasource"

// Псевдонимы типов: интерфейс datasource работает со своими типами
type Project = datasource.Project
type Issue = datasource.Issue
type Member = datasource.Member
type Status = datasource.Status
type Priority = datasource.Priority
