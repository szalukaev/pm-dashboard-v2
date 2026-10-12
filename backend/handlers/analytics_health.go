package handlers

import (
	"math"
	"net/http"
	"time"

	"pm-dashboard/utils"
)

// Health of the projects is a score from 0 to 100 over the active tasks
// (open and in testing):
//
//	100 × (1 − (0.5 × overdue share + 0.3 × Bugs share + 0.2 × no-estimate share))
//
// The weights say how much each problem costs and add up to 1, so the score
// is 0 only when every active task is overdue, in a Bugs status and has no
// estimate.
const (
	healthWeightOverdue    = 0.5
	healthWeightBugs       = 0.3
	healthWeightNoEstimate = 0.2
)

// From these scores up the health is good / needs attention; below — bad.
const (
	healthGoodFrom    = 80
	healthWarningFrom = 50
)

// HealthPart is one of the problems that lower the score.
type HealthPart struct {
	Key   string `json:"key"`
	Count int    `json:"count"`
	// Share of the active tasks, 0–100 with one decimal
	Share float64 `json:"share"`
	// Points taken off the score, with one decimal
	Penalty float64 `json:"penalty"`
}

type Health struct {
	Score  int          `json:"score"`
	Level  string       `json:"level"`
	Active int          `json:"active"`
	Parts  []HealthPart `json:"parts"`
}

func (h *AnalyticsHandler) GetHealth(w http.ResponseWriter, r *http.Request) {
	if *h.DB == nil {
		utils.Error(w, http.StatusServiceUnavailable, "DATABASE_NOT_AVAILABLE")
		return
	}

	projectFilter, args := projectIssues(r)
	today := time.Now().Format("2006-01-02")

	var active, overdue, bugs, noEstimate int
	q := `SELECT
		COUNT(*),
		COUNT(CASE WHEN due_date IS NOT NULL AND due_date < $` + itoa(len(args)+1) + ` THEN 1 END),
		COUNT(CASE WHEN ` + bugStatus("status_id") + ` THEN 1 END),
		COUNT(CASE WHEN estimated_hours IS NULL OR estimated_hours = 0 THEN 1 END)
		FROM issues WHERE ` + projectFilter + `
		AND ` + statusIn("status_id", GroupOpen, GroupTesting)
	args = append(args, today)
	if err := (*h.DB).QueryRow(q, args...).Scan(&active, &overdue, &bugs, &noEstimate); err != nil {
		utils.Error(w, http.StatusInternalServerError, "QUERY_FAILED")
		return
	}

	utils.JSON(w, http.StatusOK, healthOf(active, overdue, bugs, noEstimate))
}

// healthOf scores the active tasks. Without active tasks there is nothing to
// worry about: the score is 100.
func healthOf(active, overdue, bugs, noEstimate int) Health {
	part := func(key string, count int, weight float64) HealthPart {
		p := HealthPart{Key: key, Count: count}
		if active > 0 {
			share := float64(count) / float64(active)
			p.Share = math.Round(share*1000) / 10
			p.Penalty = math.Round(share*weight*1000) / 10
		}
		return p
	}

	health := Health{Active: active, Parts: []HealthPart{
		part("overdue", overdue, healthWeightOverdue),
		part("bugs", bugs, healthWeightBugs),
		part("no_estimate", noEstimate, healthWeightNoEstimate),
	}}

	lost := 0.0
	if active > 0 {
		lost = (healthWeightOverdue*float64(overdue) + healthWeightBugs*float64(bugs) + healthWeightNoEstimate*float64(noEstimate)) / float64(active)
	}
	health.Score = int(math.Round(100 * (1 - lost)))
	if health.Score < 0 {
		health.Score = 0
	}
	health.Level = healthLevel(health.Score)
	return health
}

func healthLevel(score int) string {
	switch {
	case score >= healthGoodFrom:
		return "good"
	case score >= healthWarningFrom:
		return "warning"
	}
	return "bad"
}
