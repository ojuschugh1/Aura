package escrow

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

// createTestSession inserts a session row so foreign key constraints are satisfied.
func createTestSession(t *testing.T, d *sql.DB, sessionID string) {
	t.Helper()
	_, err := d.Exec(`INSERT INTO sessions (id, status) VALUES (?, 'active')`, sessionID)
	if err != nil {
		t.Fatalf("insert test session: %v", err)
	}
}

// --- Escrow Lifecycle -------------------------------------------------------

func TestCreate_ReturnsPendingAction(t *testing.T) {
	d := openTestDB(t)
	createTestSession(t, d, "sess-1")
	s := New(d)

	action, err := s.Create("sess-1", "file_delete", "/tmp/foo.txt", "claude", "Delete temp file", nil)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if action.Status != "pending" {
		t.Errorf("status = %q, want pending", action.Status)
	}
	if action.ActionType != "file_delete" {
		t.Errorf("action_type = %q, want file_delete", action.ActionType)
	}
	if action.Target != "/tmp/foo.txt" {
		t.Errorf("target = %q, want /tmp/foo.txt", action.Target)
	}
	if action.Agent != "claude" {
		t.Errorf("agent = %q, want claude", action.Agent)
	}
	if action.ID == "" {
		t.Error("expected non-empty ID")
	}
	if action.CreatedAt.IsZero() {
		t.Error("expected non-zero CreatedAt")
	}
}

func TestCreate_WithParams(t *testing.T) {
	d := openTestDB(t)
	createTestSession(t, d, "sess-1")
	s := New(d)

	params := map[string]interface{}{"force": true, "branch": "main"}
	action, err := s.Create("sess-1", "git_push", "origin/main", "cursor", "Push to main", params)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Retrieve and verify params round-trip.
	got, err := s.Get(action.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Params == nil {
		t.Fatal("expected non-nil params")
	}
	if got.Params["branch"] != "main" {
		t.Errorf("params[branch] = %v, want main", got.Params["branch"])
	}
}

func TestDecide_Approve(t *testing.T) {
	d := openTestDB(t)
	createTestSession(t, d, "sess-1")
	s := New(d)

	action, err := s.Create("sess-1", "file_delete", "/tmp/foo.txt", "claude", "Delete file", nil)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := s.Decide(action.ID, "approve", "developer"); err != nil {
		t.Fatalf("Decide approve: %v", err)
	}

	got, err := s.Get(action.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Status != "approved" {
		t.Errorf("status = %q, want approved", got.Status)
	}
	if got.DecidedBy != "developer" {
		t.Errorf("decided_by = %q, want developer", got.DecidedBy)
	}
	if got.DecidedAt == nil {
		t.Error("expected DecidedAt to be set")
	}
}

func TestDecide_Deny(t *testing.T) {
	d := openTestDB(t)
	createTestSession(t, d, "sess-1")
	s := New(d)

	action, err := s.Create("sess-1", "git_push", "origin/main", "claude", "Push", nil)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := s.Decide(action.ID, "deny", "developer"); err != nil {
		t.Fatalf("Decide deny: %v", err)
	}

	got, err := s.Get(action.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Status != "denied" {
		t.Errorf("status = %q, want denied", got.Status)
	}
}

func TestDecide_InvalidDecisionReturnsError(t *testing.T) {
	d := openTestDB(t)
	createTestSession(t, d, "sess-1")
	s := New(d)

	action, err := s.Create("sess-1", "file_delete", "/tmp/foo.txt", "claude", "Delete", nil)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	err = s.Decide(action.ID, "maybe", "developer")
	if err == nil {
		t.Fatal("expected error for invalid decision, got nil")
	}
}

func TestDecide_AlreadyDecidedReturnsError(t *testing.T) {
	d := openTestDB(t)
	createTestSession(t, d, "sess-1")
	s := New(d)

	action, err := s.Create("sess-1", "file_delete", "/tmp/foo.txt", "claude", "Delete", nil)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := s.Decide(action.ID, "approve", "developer"); err != nil {
		t.Fatalf("first Decide: %v", err)
	}

	// Second decision on same action should fail.
	err = s.Decide(action.ID, "deny", "developer")
	if err == nil {
		t.Fatal("expected error when deciding already-decided action, got nil")
	}
}

func TestDecide_NonExistentIDReturnsError(t *testing.T) {
	d := openTestDB(t)
	s := New(d)

	err := s.Decide("nonexistent-id", "approve", "developer")
	if err == nil {
		t.Fatal("expected error for non-existent ID, got nil")
	}
}

// --- Timeout auto-deny ------------------------------------------------------

func TestTimeoutExpired_AutoDeniesOldPendingActions(t *testing.T) {
	d := openTestDB(t)
	createTestSession(t, d, "sess-1")
	s := New(d)

	// Override timeout to a very short duration for testing.
	s.timeout = 1 * time.Millisecond

	action, err := s.Create("sess-1", "file_delete", "/tmp/foo.txt", "claude", "Delete", nil)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Wait for the action to expire.
	time.Sleep(50 * time.Millisecond)

	n, err := s.TimeoutExpired()
	if err != nil {
		t.Fatalf("TimeoutExpired: %v", err)
	}
	if n != 1 {
		t.Errorf("timed out count = %d, want 1", n)
	}

	got, err := s.Get(action.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Status != "timeout" {
		t.Errorf("status = %q, want timeout", got.Status)
	}
	if got.DecidedBy != "timeout" {
		t.Errorf("decided_by = %q, want timeout", got.DecidedBy)
	}
}

func TestTimeoutExpired_DoesNotAffectAlreadyDecided(t *testing.T) {
	d := openTestDB(t)
	createTestSession(t, d, "sess-1")
	s := New(d)
	s.timeout = 1 * time.Millisecond

	action, err := s.Create("sess-1", "file_delete", "/tmp/foo.txt", "claude", "Delete", nil)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Approve before timeout.
	if err := s.Decide(action.ID, "approve", "developer"); err != nil {
		t.Fatalf("Decide: %v", err)
	}

	time.Sleep(50 * time.Millisecond)

	n, err := s.TimeoutExpired()
	if err != nil {
		t.Fatalf("TimeoutExpired: %v", err)
	}
	if n != 0 {
		t.Errorf("timed out count = %d, want 0 (already decided)", n)
	}

	got, err := s.Get(action.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Status != "approved" {
		t.Errorf("status = %q, want approved (unchanged)", got.Status)
	}
}

func TestTimeoutExpired_NoPendingActionsReturnsZero(t *testing.T) {
	d := openTestDB(t)
	s := New(d)

	n, err := s.TimeoutExpired()
	if err != nil {
		t.Fatalf("TimeoutExpired: %v", err)
	}
	if n != 0 {
		t.Errorf("timed out count = %d, want 0", n)
	}
}

// --- Interceptor (IsDestructive) --------------------------------------------

func TestIsDestructive_FileDelete(t *testing.T) {
	if !IsDestructive("file_delete", "/tmp/foo.txt") {
		t.Error("file_delete should be destructive")
	}
}

func TestIsDestructive_GitPush(t *testing.T) {
	if !IsDestructive("git_push", "origin/main") {
		t.Error("git_push should be destructive")
	}
}

func TestIsDestructive_GitForcePush(t *testing.T) {
	if !IsDestructive("git_force_push", "origin/main") {
		t.Error("git_force_push should be destructive")
	}
}

func TestIsDestructive_HTTPPost(t *testing.T) {
	if !IsDestructive("http", "POST") {
		t.Error("http POST should be destructive")
	}
}

func TestIsDestructive_HTTPPut(t *testing.T) {
	if !IsDestructive("http", "PUT") {
		t.Error("http PUT should be destructive")
	}
}

func TestIsDestructive_HTTPDelete(t *testing.T) {
	if !IsDestructive("http", "DELETE") {
		t.Error("http DELETE should be destructive")
	}
}

func TestIsDestructive_HTTPGetIsNotDestructive(t *testing.T) {
	if IsDestructive("http", "GET") {
		t.Error("http GET should not be destructive")
	}
}

func TestIsDestructive_ShellRm(t *testing.T) {
	if !IsDestructive("shell", "rm -rf /tmp/foo") {
		t.Error("shell rm should be destructive")
	}
}

func TestIsDestructive_ShellGitPush(t *testing.T) {
	if !IsDestructive("shell", "git push origin main") {
		t.Error("shell git push should be destructive")
	}
}

func TestIsDestructive_ShellLsIsNotDestructive(t *testing.T) {
	if IsDestructive("shell", "ls -la") {
		t.Error("shell ls should not be destructive")
	}
}

func TestIsDestructive_ReadIsNotDestructive(t *testing.T) {
	if IsDestructive("file_read", "/tmp/foo.txt") {
		t.Error("file_read should not be destructive")
	}
}

// --- Trust Window -----------------------------------------------------------

func TestTrustWindow_InactiveByDefault(t *testing.T) {
	tw := &TrustWindow{}
	if tw.IsActive("/any/path") {
		t.Error("trust window should be inactive by default")
	}
}

func TestTrustWindow_GrantActivatesForAllPaths(t *testing.T) {
	tw := &TrustWindow{}
	tw.Grant(5*time.Minute, "")

	if !tw.IsActive("/any/path") {
		t.Error("trust window should be active for any path after grant with empty path")
	}
	if !tw.IsActive("/another/path") {
		t.Error("trust window should be active for another path")
	}
}

func TestTrustWindow_ExpiresAfterDuration(t *testing.T) {
	tw := &TrustWindow{}
	tw.Grant(50*time.Millisecond, "")

	if !tw.IsActive("/any/path") {
		t.Error("trust window should be active immediately after grant")
	}

	time.Sleep(100 * time.Millisecond)

	if tw.IsActive("/any/path") {
		t.Error("trust window should be inactive after expiry")
	}
}

func TestTrustWindow_PathScopedGrant(t *testing.T) {
	tw := &TrustWindow{}
	tw.Grant(5*time.Minute, "/project/src")

	if !tw.IsActive("/project/src/main.go") {
		t.Error("trust window should be active for path within trusted directory")
	}
	if !tw.IsActive("/project/src/test/foo_test.go") {
		t.Error("trust window should be active for nested path within trusted directory")
	}
}

func TestTrustWindow_PathScopedDeniesOutsidePath(t *testing.T) {
	tw := &TrustWindow{}
	tw.Grant(5*time.Minute, "/project/src")

	if tw.IsActive("/project/config/settings.toml") {
		t.Error("trust window should not be active for path outside trusted directory")
	}
}

func TestTrustWindow_Revoke(t *testing.T) {
	tw := &TrustWindow{}
	tw.Grant(5*time.Minute, "")

	if !tw.IsActive("/any/path") {
		t.Fatal("trust window should be active before revoke")
	}

	tw.Revoke()

	if tw.IsActive("/any/path") {
		t.Error("trust window should be inactive after revoke")
	}
}

func TestTrustWindow_RevokeAndRegrant(t *testing.T) {
	tw := &TrustWindow{}
	tw.Grant(5*time.Minute, "")
	tw.Revoke()

	if tw.IsActive("/any/path") {
		t.Fatal("should be inactive after revoke")
	}

	tw.Grant(5*time.Minute, "/project/src")
	if !tw.IsActive("/project/src/main.go") {
		t.Error("should be active after re-grant")
	}
}

func TestTrustWindow_PathScopedExpiresAfterDuration(t *testing.T) {
	tw := &TrustWindow{}
	tw.Grant(50*time.Millisecond, "/project/src")

	if !tw.IsActive("/project/src/main.go") {
		t.Error("should be active immediately")
	}

	time.Sleep(100 * time.Millisecond)

	if tw.IsActive("/project/src/main.go") {
		t.Error("path-scoped trust window should expire after duration")
	}
}
