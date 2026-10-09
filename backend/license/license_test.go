package license

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// testVendor replaces the vendor keys with a fresh pair for the test and
// returns a function that issues tokens the way the vendor tool does.
func testVendor(t *testing.T) func(payload map[string]interface{}) string {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	original := vendorPublicKeys
	vendorPublicKeys = []string{hex.EncodeToString(pub)}
	t.Cleanup(func() { vendorPublicKeys = original })

	return func(payload map[string]interface{}) string {
		body, _ := json.Marshal(payload)
		return tokenEncoding.EncodeToString(body) + "." + tokenEncoding.EncodeToString(ed25519.Sign(priv, body))
	}
}

func TestVendorKeyIsWellFormed(t *testing.T) {
	for _, keyHex := range vendorPublicKeys {
		key, err := hex.DecodeString(keyHex)
		if err != nil || len(key) != ed25519.PublicKeySize {
			t.Errorf("vendor public key %q is not a 32-byte hex string", keyHex)
		}
	}
}

func TestVerify(t *testing.T) {
	issue := testVendor(t)
	token := issue(map[string]interface{}{
		"client": "ООО Ромашка", "hwid_hash": "aaa:bbb", "edition": "standard",
		"features": []string{}, "grace_period_days": 30, "issued_at": "2026-10-09",
	})

	payload, err := Verify("  " + token + "\n")
	if err != nil {
		t.Fatalf("a valid token must verify: %v", err)
	}
	if payload.Client != "ООО Ромашка" || payload.GraceDays() != 30 || payload.HWID() != (HWID{"aaa", "bbb"}) {
		t.Errorf("payload = %+v", payload)
	}

	// One changed character of the payload breaks the signature
	tampered := []byte(token)
	tampered[5] ^= 1
	if _, err := Verify(string(tampered)); !errors.Is(err, ErrSignature) && !errors.Is(err, ErrMalformed) {
		t.Errorf("a tampered token: err = %v", err)
	}

	// A token signed with another key is rejected
	if _, err := verifyWith(token, []string{hex.EncodeToString(make([]byte, 32))}); !errors.Is(err, ErrSignature) {
		t.Errorf("a foreign key: err = %v, want ErrSignature", err)
	}

	for _, bad := range []string{"", "no-dot", "a.b.c", "!!!.???"} {
		if _, err := Verify(bad); !errors.Is(err, ErrMalformed) {
			t.Errorf("Verify(%q) = %v, want ErrMalformed", bad, err)
		}
	}
}

func TestPayloadHWIDByComponents(t *testing.T) {
	issue := testVendor(t)
	token := issue(map[string]interface{}{
		"client": "c", "hwid_components": map[string]string{"machine_id_hash": "m", "board_serial_hash": "b"},
	})
	payload, err := Verify(token)
	if err != nil {
		t.Fatal(err)
	}
	if payload.HWID() != (HWID{"m", "b"}) || payload.GraceDays() != DefaultGraceDays {
		t.Errorf("hwid = %+v, grace = %d", payload.HWID(), payload.GraceDays())
	}
}

func TestHWID(t *testing.T) {
	full := HWID{MachineIDHash: "aaa", BoardSerialHash: "bbb"}
	if full.String() != "aaa:bbb" || ParseHWID(full.String()) != full {
		t.Errorf("round trip of %q failed", full.String())
	}
	half := HWID{MachineIDHash: "aaa"}
	if half.String() != "aaa:-" || ParseHWID("AAA:-") != half {
		t.Errorf("a missing component: %q / %+v", half.String(), ParseHWID("AAA:-"))
	}

	tests := []struct {
		name     string
		current  HWID
		licensed HWID
		want     bool
	}{
		{"same machine", full, full, true},
		{"OS reinstalled: board is the same", HWID{"new", "bbb"}, full, true},
		{"board replaced: machine id is the same", HWID{"aaa", "new"}, full, true},
		{"another machine", HWID{"x", "y"}, full, false},
		{"missing components never match each other", HWID{"aaa", ""}, HWID{"zzz", ""}, false},
		{"nothing known about the host", HWID{}, full, false},
	}
	for _, tt := range tests {
		if got := tt.current.Matches(tt.licensed); got != tt.want {
			t.Errorf("%s: Matches = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestParseHWIDSource(t *testing.T) {
	hwid := parseHWIDSource("# written by collect-hwid.sh\nmachine_id=0123456789abcdef\nboard_serial=PF1ABCDE\n")
	if hwid.MachineIDHash == "" || hwid.BoardSerialHash == "" || hwid.MachineIDHash == hwid.BoardSerialHash {
		t.Errorf("hwid = %+v", hwid)
	}
	if len(hwid.MachineIDHash) != 64 {
		t.Errorf("a component must be a SHA-256 in hex, got %q", hwid.MachineIDHash)
	}

	// Stubs boards report instead of a serial are not a binding
	for _, stub := range []string{"To Be Filled By O.E.M.", "Default string", "None", "0000000000", "Not Specified", ""} {
		got := parseHWIDSource("machine_id=abc\nboard_serial=" + stub + "\n")
		if got.BoardSerialHash != "" || got.MachineIDHash == "" {
			t.Errorf("stub %q: hwid = %+v", stub, got)
		}
	}
	if !parseHWIDSource("garbage").Empty() {
		t.Error("a file without identifiers gives an empty fingerprint")
	}
}

func TestGuard(t *testing.T) {
	ok := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) })
	serve := func(state State, method, path string) int {
		// No database: the status set here is what the guard sees
		s := New(nil, "")
		s.status = Status{State: state, CheckedAt: s.now()}
		rec := httptest.NewRecorder()
		s.Guard(ok).ServeHTTP(rec, httptest.NewRequest(method, path, nil))
		return rec.Code
	}

	tests := []struct {
		name   string
		state  State
		method string
		path   string
		want   int
	}{
		{"not activated: data is closed", NotActivated, "GET", "/api/tasks", http.StatusForbidden},
		{"not activated: the activation screen works", NotActivated, "POST", "/api/license/activate", http.StatusOK},
		{"not activated: who am I works", NotActivated, "GET", "/api/auth/me", http.StatusOK},
		{"active: everything works", Active, "PUT", "/api/tasks/1", http.StatusOK},
		{"grace: everything works", Grace, "PUT", "/api/tasks/1", http.StatusOK},
		{"read-only: reading works", ReadOnly, "GET", "/api/tasks", http.StatusOK},
		{"read-only: writing is blocked", ReadOnly, "PUT", "/api/tasks/1", http.StatusForbidden},
		{"read-only: deleting is blocked", ReadOnly, "DELETE", "/api/sprints/1", http.StatusForbidden},
		{"read-only: a new license can be loaded", ReadOnly, "POST", "/api/license/activate", http.StatusOK},
		{"read-only: logging out works", ReadOnly, "POST", "/api/auth/logout", http.StatusOK},
	}
	for _, tt := range tests {
		if got := serve(tt.state, tt.method, tt.path); got != tt.want {
			t.Errorf("%s: status = %d, want %d", tt.name, got, tt.want)
		}
	}
}

func TestEvaluate(t *testing.T) {
	issue := testVendor(t)
	host := HWID{MachineIDHash: "aaa", BoardSerialHash: "bbb"}
	token := issue(map[string]interface{}{"client": "c", "hwid_hash": host.String(), "grace_period_days": 14})
	now := time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC)
	daysAgo := func(n int) *time.Time { at := now.AddDate(0, 0, -n); return &at }

	t.Run("no license at all", func(t *testing.T) {
		if v := evaluate(nil, host, nil, now); v.state != NotActivated {
			t.Errorf("state = %s", v.state)
		}
	})

	t.Run("valid license on its hardware", func(t *testing.T) {
		v := evaluate(&stored{token: token}, host, nil, now)
		if v.state != Active || v.reason != "" || v.graceStarted != nil {
			t.Errorf("verdict = %+v", v)
		}
	})

	t.Run("a grace period ends when the check passes again", func(t *testing.T) {
		v := evaluate(&stored{token: token, graceStarted: daysAgo(20)}, host, nil, now)
		if v.state != Active || v.graceStarted != nil {
			t.Errorf("verdict = %+v", v)
		}
	})

	t.Run("other hardware starts the grace period", func(t *testing.T) {
		v := evaluate(&stored{token: token}, HWID{"x", "y"}, nil, now)
		if v.state != Grace || v.reason != ReasonHWIDMismatch || v.graceStarted == nil || !v.graceStarted.Equal(now) {
			t.Errorf("verdict = %+v", v)
		}
	})

	t.Run("the grace period counts from the first failure", func(t *testing.T) {
		v := evaluate(&stored{token: token, graceStarted: daysAgo(13)}, HWID{"x", "y"}, nil, now)
		if v.state != Grace || !v.graceStarted.Equal(*daysAgo(13)) {
			t.Errorf("day 13: verdict = %+v", v)
		}
		v = evaluate(&stored{token: token, graceStarted: daysAgo(14)}, HWID{"x", "y"}, nil, now)
		if v.state != ReadOnly {
			t.Errorf("day 14: state = %s, want read_only", v.state)
		}
	})

	t.Run("a damaged token gets the default grace period", func(t *testing.T) {
		v := evaluate(&stored{token: "damaged"}, host, nil, now)
		if v.state != Grace || v.reason != ReasonSignature {
			t.Errorf("verdict = %+v", v)
		}
		v = evaluate(&stored{token: "damaged", graceStarted: daysAgo(DefaultGraceDays)}, host, nil, now)
		if v.state != ReadOnly {
			t.Errorf("after the default period: state = %s", v.state)
		}
	})

	t.Run("an unreadable fingerprint is a failed check, not a pass", func(t *testing.T) {
		v := evaluate(&stored{token: token}, HWID{}, ErrHWIDUnavailable, now)
		if v.state != Grace || v.reason != ReasonHWIDUnavailable {
			t.Errorf("verdict = %+v", v)
		}
	})
}
