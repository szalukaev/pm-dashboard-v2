package handlers

import "testing"

func TestWorkerState(t *testing.T) {
	cases := []struct {
		name                     string
		enabled, running, failed bool
		want                     string
	}{
		{"on and fine", true, false, false, workerWorking},
		{"switched off", false, false, false, workerStopped},
		{"the last run failed", true, false, true, workerError},
		{"running now, whatever the last run was", true, true, true, workerStarting},
		{"switched off but started by hand", false, true, false, workerStarting},
		{"switched off after a failure: off is what matters", false, false, true, workerStopped},
	}
	for _, c := range cases {
		if got := workerState(c.enabled, c.running, c.failed); got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
}
