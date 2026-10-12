package handlers

import (
	"reflect"
	"testing"
)

func TestExceeded(t *testing.T) {
	limits := Thresholds{Overdue: 5, Bugs: 10, NoEstimate: 3}

	// Порог превышен, когда задач больше порога, а не столько же
	all := []indicatorCounts{{Overdue: 6, Bugs: 10, NoEstimate: 3}}
	want := []Exceeded{{Indicator: "overdue", Value: 6, Limit: 5}}
	if got := exceeded(all, limits); !reflect.DeepEqual(got, want) {
		t.Errorf("по всем проектам: %+v", got)
	}

	if got := exceeded([]indicatorCounts{{Overdue: 5, Bugs: 10, NoEstimate: 3}}, limits); len(got) != 0 {
		t.Errorf("ровно на пороге — не превышение: %+v", got)
	}

	// По проектам: каждое превышение со своим проектом
	projects := []indicatorCounts{
		{ProjectID: 1, ProjectName: "Albank", Overdue: 9, Bugs: 11, NoEstimate: 0},
		{ProjectID: 2, ProjectName: "ExpoBank", Overdue: 1, Bugs: 0, NoEstimate: 4},
		{ProjectID: 3, ProjectName: "SNV7"},
	}
	want = []Exceeded{
		{Indicator: "overdue", Value: 9, Limit: 5, ProjectID: 1, ProjectName: "Albank"},
		{Indicator: "bugs", Value: 11, Limit: 10, ProjectID: 1, ProjectName: "Albank"},
		{Indicator: "no_estimate", Value: 4, Limit: 3, ProjectID: 2, ProjectName: "ExpoBank"},
	}
	if got := exceeded(projects, limits); !reflect.DeepEqual(got, want) {
		t.Errorf("по проектам: %+v", got)
	}

	// Порог 0 — превышение уже при одной задаче
	if got := exceeded([]indicatorCounts{{Bugs: 1}}, Thresholds{Overdue: 0, Bugs: 0, NoEstimate: 0}); len(got) != 1 || got[0].Indicator != "bugs" {
		t.Errorf("порог 0: %+v", got)
	}
	if got := exceeded(nil, limits); got == nil || len(got) != 0 {
		t.Errorf("без данных — пустой список, а не null: %+v", got)
	}
}

func TestThresholds(t *testing.T) {
	// Пороги из ТЗ: просрочено > 5, Bugs > 10, без оценки > 3
	if def := defaultThresholds(); def != (Thresholds{Overdue: 5, Bugs: 10, NoEstimate: 3}) {
		t.Errorf("стандартные пороги: %+v", def)
	}
	if !(Thresholds{}).valid() || !(Thresholds{Overdue: maxThreshold}).valid() {
		t.Error("пороги от 0 до наибольшего допустимы")
	}
	if (Thresholds{Bugs: -1}).valid() || (Thresholds{NoEstimate: maxThreshold + 1}).valid() {
		t.Error("отрицательный и слишком большой порог недопустимы")
	}

	got := parseThresholds([]byte(`{"overdue": 1, "bugs": 2, "no_estimate": 0, "per_project": true}`))
	if got != (Thresholds{Overdue: 1, Bugs: 2, NoEstimate: 0, PerProject: true}) {
		t.Errorf("сохранённые пороги: %+v", got)
	}
	// Не заданное поле остаётся стандартным
	if got := parseThresholds([]byte(`{"overdue": 1}`)); got.Bugs != 10 || got.NoEstimate != 3 {
		t.Errorf("частично заданные пороги: %+v", got)
	}
	if got := parseThresholds([]byte(`{"overdue": -5}`)); got != defaultThresholds() {
		t.Errorf("негодные пороги заменяются стандартными: %+v", got)
	}
	if got := parseThresholds(nil); got != defaultThresholds() {
		t.Errorf("пустые пороги: %+v", got)
	}
}
