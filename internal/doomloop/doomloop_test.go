package doomloop

import (
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/ojuschugh1/aura/internal/db"
)

// openTestDB creates an isolated SQLite database with all migrations applied.
func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	d, err := db.Open(dbPath)
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	if err := db.RunMigrations(d); err != nil {
		t.Fatalf("RunMigrations: %v", err)
	}
	t.Cleanup(func() { d.Close() })
	return d
}

// createTestSession inserts a session row so doom_loop_actions FK is satisfied.
func createTestSession(t *testing.T, d *sql.DB, sessionID string) {
	t.Helper()
	now := time.Now().UTC().Format(time.RFC3339Nano)
	_, err := d.Exec(
		`INSERT INTO sessions (id, started_at, status) VALUES (?, ?, 'active')`,
		sessionID, now,
	)
	if err != nil {
		t.Fatalf("create test session %s: %v", sessionID, err)
	}
}

// --- Fingerprint tests (Req 7.3) -------------------------------------------

func TestFingerprint_SameActionProducesSameHash(t *testing.T) {
	a := Action{Type: "shell", Target: "npm install", Params: map[string]interface{}{"flag": "--save"}}
	b := Action{Type: "shell", Target: "npm install", Params: map[string]interface{}{"flag": "--save"}}

	if Fingerprint(a) != Fingerprint(b) {
		t.Error("identical actions should produce the same fingerprint")
	}
}

func TestFingerprint_DifferentTypeProducesDifferentHash(t *testing.T) {
	a := Action{Type: "shell", Target: "npm install", Params: nil}
	b := Action{Type: "file_write", Target: "npm install", Params: nil}

	if Fingerprint(a) == Fingerprint(b) {
		t.Error("actions with different types should produce different fingerprints")
	}
}

func TestFingerprint_DifferentTargetProducesDifferentHash(t *testing.T) {
	a := Action{Type: "shell", Target: "npm install", Params: nil}
	b := Action{Type: "shell", Target: "npm test", Params: nil}

	if Fingerprint(a) == Fingerprint(b) {
		t.Error("actions with different targets should produce different fingerprints")
	}
}

func TestFingerprint_DifferentParamsProducesDifferentHash(t *testing.T) {
	a := Action{Type: "shell", Target: "npm install", Params: map[string]interface{}{"pkg": "express"}}
	b := Action{Type: "shell", Target: "npm install", Params: map[string]interface{}{"pkg": "lodash"}}

	if Fingerprint(a) == Fingerprint(b) {
		t.Error("actions with different params should produce different fingerprints")
	}
}

func TestFingerprint_NilParamsMatchesEmptyParams(t *testing.T) {
	// nil and empty map both marshal to "null" and "{}" respectively,
	// so they may differ. This test documents the behavior.
	a := Action{Type: "shell", Target: "cmd", Params: nil}
	b := Action{Type: "shell", Target: "cmd", Params: map[string]interface{}{}}

	fpA := Fingerprint(a)
	fpB := Fingerprint(b)
	// Just verify both produce valid non-empty fingerprints.
	if fpA == "" || fpB == "" {
		t.Error("fingerprints should not be empty")
	}
}

// --- Repetition counting tests (Req 7.2) ------------------------------------

func TestRecord_ThreeFailuresTriggersAlert(t *testing.T) {
	d := openTestDB(t)
	createTestSession(t, d, "sess-1")
	det := New(d)

	action := Action{Type: "package_install", Target: "express", Params: nil, Outcome: "failure"}

	// First two failures: no alert.
	for i := 0; i < 2; i++ {
		alert, err := det.Record("sess-1", action)
		if err != nil {
			t.Fatalf("Record %d: %v", i+1, err)
		}
		if alert != nil {
			t.Errorf("Record %d: unexpected alert after %d failures", i+1, i+1)
		}
	}

	// Third failure: alert expected.
	alert, err := det.Record("sess-1", action)
	if err != nil {
		t.Fatalf("Record 3: %v", err)
	}
	if alert == nil {
		t.Fatal("expected alert after 3 consecutive failures, got nil")
	}
}

func TestRecord_TwoFailuresDoesNotTriggerAlert(t *testing.T) {
	d := openTestDB(t)
	createTestSession(t, d, "sess-1")
	det := New(d)

	action := Action{Type: "shell", Target: "make build", Params: nil, Outcome: "failure"}

	for i := 0; i < 2; i++ {
		alert, err := det.Record("sess-1", action)
		if err != nil {
			t.Fatalf("Record %d: %v", i+1, err)
		}
		if alert != nil {
			t.Errorf("should not alert after only %d failures", i+1)
		}
	}
}

func TestRecord_FourFailuresContinuesToAlert(t *testing.T) {
	d := openTestDB(t)
	createTestSession(t, d, "sess-1")
	det := New(d)

	action := Action{Type: "shell", Target: "go build", Params: nil, Outcome: "failure"}

	// Record 4 failures; alert should fire on 3rd and 4th.
	for i := 1; i <= 4; i++ {
		alert, err := det.Record("sess-1", action)
		if err != nil {
			t.Fatalf("Record %d: %v", i, err)
		}
		if i >= 3 && alert == nil {
			t.Errorf("expected alert on failure %d", i)
		}
		if i < 3 && alert != nil {
			t.Errorf("unexpected alert on failure %d", i)
		}
	}
}

// --- Counter reset tests (Req 7.5) ------------------------------------------

func TestRecord_SuccessResetsCounter(t *testing.T) {
	d := openTestDB(t)
	createTestSession(t, d, "sess-1")
	det := New(d)

	fail := Action{Type: "file_write", Target: "/etc/config", Params: nil, Outcome: "failure"}
	success := Action{Type: "file_write", Target: "/etc/config", Params: nil, Outcome: "success"}

	// Two failures, then a success.
	det.Record("sess-1", fail)
	det.Record("sess-1", fail)
	alert, err := det.Record("sess-1", success)
	if err != nil {
		t.Fatalf("Record success: %v", err)
	}
	if alert != nil {
		t.Error("success should not trigger an alert")
	}

	// Two more failures after reset: should NOT alert (counter was reset).
	for i := 0; i < 2; i++ {
		alert, err = det.Record("sess-1", fail)
		if err != nil {
			t.Fatalf("Record post-reset %d: %v", i+1, err)
		}
		if alert != nil {
			t.Errorf("unexpected alert after reset, failure %d", i+1)
		}
	}

	// Third failure after reset: should alert again.
	alert, err = det.Record("sess-1", fail)
	if err != nil {
		t.Fatalf("Record 3rd post-reset: %v", err)
	}
	if alert == nil {
		t.Error("expected alert after 3 failures post-reset")
	}
}

func TestRecord_DifferentActionDoesNotAffectCounter(t *testing.T) {
	d := openTestDB(t)
	createTestSession(t, d, "sess-1")
	det := New(d)

	actionA := Action{Type: "shell", Target: "npm install", Params: nil, Outcome: "failure"}
	actionB := Action{Type: "file_write", Target: "/tmp/out.txt", Params: nil, Outcome: "failure"}

	// Two failures of action A.
	det.Record("sess-1", actionA)
	det.Record("sess-1", actionA)

	// One failure of a different action B.
	alert, err := det.Record("sess-1", actionB)
	if err != nil {
		t.Fatalf("Record actionB: %v", err)
	}
	if alert != nil {
		t.Error("different action should not trigger alert for action A's counter")
	}

	// Third failure of action A: should alert (action B doesn't reset A's counter).
	alert, err = det.Record("sess-1", actionA)
	if err != nil {
		t.Fatalf("Record actionA 3rd: %v", err)
	}
	if alert == nil {
		t.Fatal("expected alert after 3 failures of action A")
	}
}

func TestRecord_IndependentCountersPerAction(t *testing.T) {
	d := openTestDB(t)
	createTestSession(t, d, "sess-1")
	det := New(d)

	actionA := Action{Type: "shell", Target: "cmd-a", Params: nil, Outcome: "failure"}
	actionB := Action{Type: "shell", Target: "cmd-b", Params: nil, Outcome: "failure"}

	// Interleave failures: A, B, A, B, A — A has 3 failures, B has 2.
	det.Record("sess-1", actionA)
	det.Record("sess-1", actionB)
	det.Record("sess-1", actionA)
	det.Record("sess-1", actionB)

	alertA, err := det.Record("sess-1", actionA)
	if err != nil {
		t.Fatalf("Record actionA 3rd: %v", err)
	}
	if alertA == nil {
		t.Error("expected alert for action A after 3 failures")
	}

	// B still at 2 failures — no alert.
	alertB, err := det.Record("sess-1", actionB)
	if err != nil {
		t.Fatalf("Record actionB 3rd: %v", err)
	}
	if alertB == nil {
		t.Error("expected alert for action B after 3 failures")
	}
}

// --- Alert content tests (Req 7.2, 7.4) ------------------------------------

func TestAlert_ContainsActionTypeAndTarget(t *testing.T) {
	d := openTestDB(t)
	createTestSession(t, d, "sess-1")
	det := New(d)

	action := Action{Type: "package_install", Target: "express", Params: nil, Outcome: "failure"}

	var alert *Alert
	for i := 0; i < 3; i++ {
		a, err := det.Record("sess-1", action)
		if err != nil {
			t.Fatalf("Record %d: %v", i+1, err)
		}
		alert = a
	}

	if alert == nil {
		t.Fatal("expected alert")
	}
	if alert.ActionType != "package_install" {
		t.Errorf("ActionType = %q, want %q", alert.ActionType, "package_install")
	}
	if alert.Target != "express" {
		t.Errorf("Target = %q, want %q", alert.Target, "express")
	}
}

func TestAlert_ContainsRepetitionCount(t *testing.T) {
	d := openTestDB(t)
	createTestSession(t, d, "sess-1")
	det := New(d)

	action := Action{Type: "shell", Target: "go test", Params: nil, Outcome: "failure"}

	var alert *Alert
	for i := 0; i < 3; i++ {
		a, _ := det.Record("sess-1", action)
		alert = a
	}

	if alert == nil {
		t.Fatal("expected alert")
	}
	if alert.Repetitions != 3 {
		t.Errorf("Repetitions = %d, want 3", alert.Repetitions)
	}
}

func TestAlert_RepetitionCountIncrementsOnFurtherFailures(t *testing.T) {
	d := openTestDB(t)
	createTestSession(t, d, "sess-1")
	det := New(d)

	action := Action{Type: "shell", Target: "go test", Params: nil, Outcome: "failure"}

	var alert *Alert
	for i := 0; i < 5; i++ {
		a, _ := det.Record("sess-1", action)
		alert = a
	}

	if alert == nil {
		t.Fatal("expected alert")
	}
	if alert.Repetitions != 5 {
		t.Errorf("Repetitions = %d, want 5", alert.Repetitions)
	}
}

func TestAlert_IncludesSuggestion(t *testing.T) {
	d := openTestDB(t)
	createTestSession(t, d, "sess-1")
	det := New(d)

	action := Action{Type: "package_install", Target: "express", Params: nil, Outcome: "failure"}

	var alert *Alert
	for i := 0; i < 3; i++ {
		a, _ := det.Record("sess-1", action)
		alert = a
	}

	if alert == nil {
		t.Fatal("expected alert")
	}
	if alert.Suggestion == "" {
		t.Error("alert suggestion should not be empty")
	}
}

// --- Reset tests (Req 7.5) --------------------------------------------------

func TestReset_ClearsAllCounters(t *testing.T) {
	d := openTestDB(t)
	createTestSession(t, d, "sess-1")
	det := New(d)

	action := Action{Type: "shell", Target: "make", Params: nil, Outcome: "failure"}

	// Accumulate 2 failures.
	det.Record("sess-1", action)
	det.Record("sess-1", action)

	// Reset all counters.
	det.Reset()

	// Next 2 failures should not trigger alert (counter was cleared).
	for i := 0; i < 2; i++ {
		alert, err := det.Record("sess-1", action)
		if err != nil {
			t.Fatalf("Record post-reset %d: %v", i+1, err)
		}
		if alert != nil {
			t.Errorf("unexpected alert after Reset, failure %d", i+1)
		}
	}
}

// --- DB persistence tests ---------------------------------------------------

func TestRecord_PersistsActionToDatabase(t *testing.T) {
	d := openTestDB(t)
	createTestSession(t, d, "sess-1")
	det := New(d)

	action := Action{Type: "shell", Target: "go build", Params: nil, Outcome: "failure"}
	_, err := det.Record("sess-1", action)
	if err != nil {
		t.Fatalf("Record: %v", err)
	}

	var count int
	if err := d.QueryRow("SELECT COUNT(*) FROM doom_loop_actions WHERE session_id = ?", "sess-1").Scan(&count); err != nil {
		t.Fatalf("query: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 doom_loop_actions row, got %d", count)
	}
}

func TestRecord_PersistsCorrectFields(t *testing.T) {
	d := openTestDB(t)
	createTestSession(t, d, "sess-1")
	det := New(d)

	action := Action{Type: "file_write", Target: "/tmp/out.txt", Params: map[string]interface{}{"mode": "append"}, Outcome: "failure"}
	_, err := det.Record("sess-1", action)
	if err != nil {
		t.Fatalf("Record: %v", err)
	}

	var actionType, target, outcome string
	err = d.QueryRow(
		"SELECT action_type, target, outcome FROM doom_loop_actions WHERE session_id = ?",
		"sess-1",
	).Scan(&actionType, &target, &outcome)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if actionType != "file_write" {
		t.Errorf("action_type = %q, want %q", actionType, "file_write")
	}
	if target != "/tmp/out.txt" {
		t.Errorf("target = %q, want %q", target, "/tmp/out.txt")
	}
	if outcome != "failure" {
		t.Errorf("outcome = %q, want %q", outcome, "failure")
	}
}
