package handlers

import (
	"math"
	"net/http"
	"time"

	"pm-dashboard/utils"
)

// Здоровье проектов — оценка от 0 до 100 по активным задачам (открытым и в
// тестировании):
//
//	100 × (1 − (0,5 × доля просроченных + 0,3 × доля Bugs + 0,2 × доля без оценки))
//
// Веса показывают, во что обходится каждая проблема, и в сумме дают 1,
// поэтому оценка равна 0, только когда каждая активная задача просрочена,
// стоит в статусе Bugs и не имеет оценки.
const (
	healthWeightOverdue    = 0.5
	healthWeightBugs       = 0.3
	healthWeightNoEstimate = 0.2
)

// Начиная с этих оценок здоровье хорошее / требует внимания; ниже — плохое.
const (
	healthGoodFrom    = 80
	healthWarningFrom = 50
)

// HealthPart — одна из проблем, снижающих оценку.
type HealthPart struct {
	Key   string `json:"key"`
	Count int    `json:"count"`
	// Доля активных задач, 0–100 с одним знаком после запятой
	Share float64 `json:"share"`
	// Сколько баллов снято с оценки, с одним знаком после запятой
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

	projectFilter, args, ok := periodIssues(r)
	if !ok {
		utils.Error(w, http.StatusBadRequest, "INVALID_PERIOD")
		return
	}
	today := time.Now().Format("2006-01-02")

	var active, overdue, bugs, noEstimate int
	q := `SELECT
		COUNT(*),
		COUNT(CASE WHEN due_date IS NOT NULL AND due_date < $` + itoa(len(args)+1) + ` THEN 1 END),
		COUNT(CASE WHEN ` + bugStatus("status_id") + ` THEN 1 END),
		COUNT(CASE WHEN (estimated_hours IS NULL OR estimated_hours = 0) AND `+statusIn("status_id", GroupOpen)+` THEN 1 END)
		FROM issues WHERE ` + projectFilter + `
		AND ` + statusIn("status_id", GroupOpen, GroupTesting)
	args = append(args, today)
	if err := (*h.DB).QueryRow(q, args...).Scan(&active, &overdue, &bugs, &noEstimate); err != nil {
		utils.Error(w, http.StatusInternalServerError, "QUERY_FAILED")
		return
	}

	utils.JSON(w, http.StatusOK, healthOf(active, overdue, bugs, noEstimate))
}

// healthOf оценивает активные задачи. Без активных задач беспокоиться не о
// чем: оценка равна 100.
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
