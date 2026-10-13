package handlers

import "testing"

func TestHealthOf(t *testing.T) {
	cases := []struct {
		name                              string
		active, overdue, bugs, noEstimate int
		score                             int
		level                             string
	}{
		{"nothing is wrong", 40, 0, 0, 0, 100, "good"},
		{"no active tasks", 0, 0, 0, 0, 100, "good"},
		{"everything is wrong", 10, 10, 10, 10, 0, "bad"},
		// 100 × (1 − (0.5×0.2 + 0.3×0.1 + 0.2×0.5)) = 77
		{"a mix of problems", 10, 2, 1, 5, 77, "warning"},
		{"only overdue tasks cost half", 10, 10, 0, 0, 50, "warning"},
		{"only Bugs cost 30 points", 10, 0, 10, 0, 70, "warning"},
		{"only missing estimates cost 20 points", 10, 0, 0, 10, 80, "good"},
		// 100 × (1 − (0.5×0.6 + 0.3×0.4 + 0.2×0.5)) = 48
		{"below the warning threshold", 10, 6, 4, 5, 48, "bad"},
		// 100 × (1 − 0.5 × 1/3) = 83.33
		{"the score is rounded", 3, 1, 0, 0, 83, "good"},
	}
	for _, c := range cases {
		got := healthOf(c.active, c.overdue, c.bugs, c.noEstimate)
		if got.Score != c.score || got.Level != c.level {
			t.Errorf("%s: score %d (%s), want %d (%s)", c.name, got.Score, got.Level, c.score, c.level)
		}
		if got.Active != c.active || len(got.Parts) != 3 {
			t.Errorf("%s: active %d, parts %d", c.name, got.Active, len(got.Parts))
		}
	}
}

// Слагаемые объясняют оценку: их штрафы в сумме дают потерянные баллы.
func TestHealthParts(t *testing.T) {
	got := healthOf(10, 2, 1, 5)
	want := []HealthPart{
		{Key: "overdue", Count: 2, Share: 20, Penalty: 10},
		{Key: "bugs", Count: 1, Share: 10, Penalty: 3},
		{Key: "no_estimate", Count: 5, Share: 50, Penalty: 10},
	}
	for i, p := range got.Parts {
		if p != want[i] {
			t.Errorf("part %d: got %+v, want %+v", i, p, want[i])
		}
	}

	for _, p := range healthOf(0, 0, 0, 0).Parts {
		if p.Share != 0 || p.Penalty != 0 {
			t.Errorf("without active tasks nothing is lost: %+v", p)
		}
	}
}

func TestHealthWeights(t *testing.T) {
	if sum := healthWeightOverdue + healthWeightBugs + healthWeightNoEstimate; sum != 1 {
		t.Errorf("the weights must add up to 1, got %v", sum)
	}
}

func TestHealthLevel(t *testing.T) {
	cases := map[int]string{100: "good", 80: "good", 79: "warning", 50: "warning", 49: "bad", 0: "bad"}
	for score, want := range cases {
		if got := healthLevel(score); got != want {
			t.Errorf("healthLevel(%d) = %s, want %s", score, got, want)
		}
	}
}
