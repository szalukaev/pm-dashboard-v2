package handlers

import (
	"net/url"
	"testing"
)

func TestBuildDSNEscapesCredentials(t *testing.T) {
	req := dbTestRequest{Host: "db.local", Port: "", User: "pm@corp", Password: "p@ss:w/rd?#1", DBName: "pm_dashboard"}
	u, err := url.Parse(buildDSN(req))
	if err != nil {
		t.Fatalf("DSN must be a valid URL: %v", err)
	}
	if u.Host != "db.local:5432" {
		t.Errorf("host = %q, want db.local:5432", u.Host)
	}
	if u.User.Username() != req.User {
		t.Errorf("user = %q, want %q", u.User.Username(), req.User)
	}
	if pass, _ := u.User.Password(); pass != req.Password {
		t.Errorf("password = %q, want %q", pass, req.Password)
	}
	if u.Path != "/pm_dashboard" {
		t.Errorf("path = %q, want /pm_dashboard", u.Path)
	}
	if m, _ := url.Parse(buildMaintDSN(req)); m.Path != "/postgres" {
		t.Errorf("maintenance path = %q, want /postgres", m.Path)
	}
}

func TestDBNamePattern(t *testing.T) {
	valid := []string{"pm_dashboard", "pm-dashboard2", "DB1"}
	invalid := []string{"", `pm"; DROP DATABASE x; --`, "pm dashboard", "pm/dash", "база"}
	for _, name := range valid {
		if !dbNamePattern.MatchString(name) {
			t.Errorf("%q must be accepted", name)
		}
	}
	for _, name := range invalid {
		if dbNamePattern.MatchString(name) {
			t.Errorf("%q must be rejected", name)
		}
	}
}
