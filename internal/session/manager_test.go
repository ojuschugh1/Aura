package session

import (
	"database/sql"
	"path/filepath"
	"regexp"
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

// uuidV4Re matches a UUID v4 string (8-4-4-4-12 hex format).
var uuidV4Re = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

// --- Create -----------------------------------------------------------------

func TestCreate_ReturnsValidSession(t *testing.T) {
	d := openTestDB(t)
	m := New(d)

	s, err := m.Create()
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if !uuidV4Re.MatchString(s.ID) {
		t.Errorf("expected UUID v4, got %q", s.ID)
	}
	if s.Status != "active" {
		t.Errorf("expected status=active, got %q", s.Status)
	}
	if s.StartedAt.IsZero() {
		t.Error("expected non-zero StartedAt")
	}
	if s.EndedAt != nil {
		t.Error("expected EndedAt to be nil for a new session")
	}
}

func TestCreate_SetsCurrent(t *testing.T) {
	d := openTestDB(t)
	m := New(d)

	s, err := m.Create()
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	cur := m.Current()
	if cur == nil {
		t.Fatal("Current() returned nil after Create")
	}
	if cur.ID != s.ID {
		t.Errorf("Current().ID = %q, want %q", cur.ID, s.ID)
	}
}

func TestCreate_MultipleSessionsUpdatesCurrentToLatest(t *testing.T) {
	d := openTestDB(t)
	m := New(d)

	_, err := m.Create()
	if err != nil {
		t.Fatalf("Create first: %v", err)
	}

	s2, err := m.Create()
	if err != nil {
		t.Fatalf("Create second: %v", err)
	}

	cur := m.Current()
	if cur == nil {
		t.Fatal("Current() returned nil")
	}
	if cur.ID != s2.ID {
		t.Errorf("Current().ID = %q, want latest %q", cur.ID, s2.ID)
	}
}

// --- Get --------------------------------------------------------------------

func TestGet_RetrievesCreatedSession(t *testing.T) {
	d := openTestDB(t)
	m := New(d)

	created, err := m.Create()
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	got, err := m.Get(created.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}

	if got.ID != created.ID {
		t.Errorf("Get().ID = %q, want %q", got.ID, created.ID)
	}
	if got.Status != "active" {
		t.Errorf("Get().Status = %q, want active", got.Status)
	}
}

func TestGet_NonExistentSessionReturnsError(t *testing.T) {
	d := openTestDB(t)
	m := New(d)

	_, err := m.Get("does-not-exist")
	if err == nil {
		t.Fatal("expected error for non-existent session, got nil")
	}
}

// --- Current ----------------------------------------------------------------

func TestCurrent_NilWhenNoSessionCreated(t *testing.T) {
	d := openTestDB(t)
	m := New(d)

	if cur := m.Current(); cur != nil {
		t.Errorf("expected nil Current(), got %+v", cur)
	}
}

// --- End --------------------------------------------------------------------

func TestEnd_MarksSessionCompleted(t *testing.T) {
	d := openTestDB(t)
	m := New(d)

	s, err := m.Create()
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := m.End(s.ID); err != nil {
		t.Fatalf("End: %v", err)
	}

	got, err := m.Get(s.ID)
	if err != nil {
		t.Fatalf("Get after End: %v", err)
	}
	if got.Status != "completed" {
		t.Errorf("status = %q, want completed", got.Status)
	}
	if got.EndedAt == nil {
		t.Error("expected EndedAt to be set after End")
	}
}

func TestEnd_ClearsCurrentIfMatchingSession(t *testing.T) {
	d := openTestDB(t)
	m := New(d)

	s, err := m.Create()
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := m.End(s.ID); err != nil {
		t.Fatalf("End: %v", err)
	}

	if cur := m.Current(); cur != nil {
		t.Errorf("expected nil Current() after ending current session, got %+v", cur)
	}
}

func TestEnd_DoesNotClearCurrentIfDifferentSession(t *testing.T) {
	d := openTestDB(t)
	m := New(d)

	s1, err := m.Create()
	if err != nil {
		t.Fatalf("Create first: %v", err)
	}

	s2, err := m.Create()
	if err != nil {
		t.Fatalf("Create second: %v", err)
	}

	// End the first session; current should still be s2.
	if err := m.End(s1.ID); err != nil {
		t.Fatalf("End: %v", err)
	}

	cur := m.Current()
	if cur == nil {
		t.Fatal("Current() should not be nil after ending a different session")
	}
	if cur.ID != s2.ID {
		t.Errorf("Current().ID = %q, want %q", cur.ID, s2.ID)
	}
}

func TestEnd_NonExistentSessionReturnsError(t *testing.T) {
	d := openTestDB(t)
	m := New(d)

	err := m.End("does-not-exist")
	if err == nil {
		t.Fatal("expected error when ending non-existent session, got nil")
	}
}

func TestEnd_SetsReasonableEndTimestamp(t *testing.T) {
	d := openTestDB(t)
	m := New(d)

	s, err := m.Create()
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	before := time.Now().UTC()
	if err := m.End(s.ID); err != nil {
		t.Fatalf("End: %v", err)
	}
	after := time.Now().UTC()

	got, err := m.Get(s.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.EndedAt == nil {
		t.Fatal("EndedAt is nil")
	}
	if got.EndedAt.Before(before.Add(-time.Second)) || got.EndedAt.After(after.Add(time.Second)) {
		t.Errorf("EndedAt %v not within expected range [%v, %v]", got.EndedAt, before, after)
	}
}

// --- List -------------------------------------------------------------------

func TestList_EmptyWhenNoSessions(t *testing.T) {
	d := openTestDB(t)
	m := New(d)

	sessions, err := m.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(sessions) != 0 {
		t.Errorf("expected 0 sessions, got %d", len(sessions))
	}
}

func TestList_ReturnsAllSessions(t *testing.T) {
	d := openTestDB(t)
	m := New(d)

	for i := 0; i < 3; i++ {
		if _, err := m.Create(); err != nil {
			t.Fatalf("Create %d: %v", i, err)
		}
	}

	sessions, err := m.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(sessions) != 3 {
		t.Errorf("expected 3 sessions, got %d", len(sessions))
	}
}

func TestList_IncludesActiveAndEndedSessions(t *testing.T) {
	d := openTestDB(t)
	m := New(d)

	s1, err := m.Create()
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := m.End(s1.ID); err != nil {
		t.Fatalf("End: %v", err)
	}

	if _, err := m.Create(); err != nil {
		t.Fatalf("Create second: %v", err)
	}

	sessions, err := m.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(sessions) != 2 {
		t.Fatalf("expected 2 sessions, got %d", len(sessions))
	}

	statusCounts := map[string]int{}
	for _, s := range sessions {
		statusCounts[s.Status]++
	}
	if statusCounts["active"] != 1 {
		t.Errorf("expected 1 active session, got %d", statusCounts["active"])
	}
	if statusCounts["completed"] != 1 {
		t.Errorf("expected 1 completed session, got %d", statusCounts["completed"])
	}
}

func TestList_OrderedByStartTimeDescending(t *testing.T) {
	d := openTestDB(t)
	m := New(d)

	var ids []string
	for i := 0; i < 3; i++ {
		s, err := m.Create()
		if err != nil {
			t.Fatalf("Create %d: %v", i, err)
		}
		ids = append(ids, s.ID)
		// Small sleep to ensure distinct timestamps.
		time.Sleep(10 * time.Millisecond)
	}

	sessions, err := m.List()
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(sessions) != 3 {
		t.Fatalf("expected 3 sessions, got %d", len(sessions))
	}

	// Most recent first (ids[2] was created last).
	if sessions[0].ID != ids[2] {
		t.Errorf("first listed session = %q, want most recent %q", sessions[0].ID, ids[2])
	}
	if sessions[2].ID != ids[0] {
		t.Errorf("last listed session = %q, want oldest %q", sessions[2].ID, ids[0])
	}
}

// --- SetOnEndHook -----------------------------------------------------------

func TestSetOnEndHook_CalledOnEnd(t *testing.T) {
	d := openTestDB(t)
	m := New(d)

	var hookCalledWith string
	done := make(chan struct{})
	m.SetOnEndHook(func(sessionID string) {
		hookCalledWith = sessionID
		close(done)
	})

	s, err := m.Create()
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := m.End(s.ID); err != nil {
		t.Fatalf("End: %v", err)
	}

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("hook was not called within 1 second")
	}

	if hookCalledWith != s.ID {
		t.Errorf("hook called with %q, want %q", hookCalledWith, s.ID)
	}
}

func TestSetOnEndHook_NotCalledOnError(t *testing.T) {
	d := openTestDB(t)
	m := New(d)

	hookCalled := false
	m.SetOnEndHook(func(sessionID string) {
		hookCalled = true
	})

	// End a non-existent session — should error and not call the hook.
	err := m.End("does-not-exist")
	if err == nil {
		t.Fatal("expected error for non-existent session")
	}

	if hookCalled {
		t.Error("hook should not be called when End fails")
	}
}

func TestSetOnEndHook_NilHookDoesNotPanic(t *testing.T) {
	d := openTestDB(t)
	m := New(d)

	// No hook set — End should not panic.
	s, err := m.Create()
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := m.End(s.ID); err != nil {
		t.Fatalf("End: %v", err)
	}
}
