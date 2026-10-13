package handlers

import "testing"

func TestParseDSN(t *testing.T) {
	// Пароль со служебными знаками возвращается таким, каким был введён
	want := dbTestRequest{Host: "db.local", Port: "5433", User: "pm", Password: "p@ss:w/rd?", DBName: "pm_dashboard"}
	if got := parseDSN(buildDSN(want)); got != want {
		t.Errorf("parseDSN(buildDSN(x)) = %+v, want %+v", got, want)
	}
	if got := parseDSN("postgres://pm@db.local/pm").Port; got != "5432" {
		t.Errorf("default port = %q, want 5432", got)
	}
	if got := parseDSN("host=db user=pm"); got != (dbTestRequest{}) {
		t.Errorf("non-URL DSN parsed as %+v, want empty", got)
	}
}

func TestCheckDatabaseFields(t *testing.T) {
	cases := []struct {
		req  dbTestRequest
		want string
	}{
		{dbTestRequest{Host: "", Port: "5432", User: "pm", DBName: "pm"}, "DB_FIELDS_REQUIRED"},
		{dbTestRequest{Host: "db", Port: "5432", User: "", DBName: "pm"}, "DB_FIELDS_REQUIRED"},
		{dbTestRequest{Host: "db", Port: "70000", User: "pm", DBName: "pm"}, "DB_FIELDS_REQUIRED"},
		{dbTestRequest{Host: "db", Port: "abc", User: "pm", DBName: "pm"}, "DB_FIELDS_REQUIRED"},
		{dbTestRequest{Host: "db", Port: "5432", User: "pm", DBName: `pm"; DROP`}, "INVALID_DB_NAME"},
	}
	for _, c := range cases {
		if got, _ := checkDatabase(c.req); got != c.want {
			t.Errorf("checkDatabase(%+v) = %q, want %q", c.req, got, c.want)
		}
	}
}
