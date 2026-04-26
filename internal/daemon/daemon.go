package daemon

import (
	"database/sql"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/ojuschugh1/aura/internal/autocapture"
	"github.com/ojuschugh1/aura/internal/db"
	"github.com/ojuschugh1/aura/internal/memory"
	"github.com/ojuschugh1/aura/internal/session"
	"github.com/ojuschugh1/aura/internal/wiki"
)

// EnvDaemon is set to "1" when the process is running as the background daemon.
const EnvDaemon = "AURA_DAEMON"

// DefaultPort is the MCP server port used when none is specified.
const DefaultPort = 7777

// MCPServer is a placeholder interface for the MCP server (implemented in task 7).
type MCPServer interface {
	Start(port int) error
	Stop() error
	Port() int
}

// noopMCP is a placeholder that satisfies MCPServer until task 7.
type noopMCP struct{ port int }

func (n *noopMCP) Start(port int) error { n.port = port; return nil }
func (n *noopMCP) Stop() error          { return nil }
func (n *noopMCP) Port() int            { return n.port }

// Daemon holds the runtime state of a running Aura daemon.
type Daemon struct {
	dir    string
	db     *sql.DB
	mcp    MCPServer
	port   int
	sessID string
}

// StatusInfo is returned by Status().
type StatusInfo struct {
	Running     bool
	PID         int
	Port        int
	MemoryCount int64
	SessionID   string
}

// IsDaemonProcess reports whether the current process is the daemon (not the CLI).
func IsDaemonProcess() bool {
	return os.Getenv(EnvDaemon) == "1"
}

// Start forks the current binary as a background daemon process.
// It returns immediately in the CLI process; the daemon process calls RunDaemon.
func Start(dir string, port int) error {
	// Ensure .aura/ directory exists.
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create aura dir: %w", err)
	}

	// Refuse to start if a non-stale lock file already exists.
	if info, err := ReadLockFile(dir); err == nil && !IsStale(info) {
		return fmt.Errorf("daemon already running (pid %d)", info.PID)
	}

	// Fork the current binary with AURA_DAEMON=1.
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve executable: %w", err)
	}

	logPath := filepath.Join(dir, "daemon.log")
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("open daemon log: %w", err)
	}

	cmd := exec.Command(exe)
	cmd.Env = append(os.Environ(),
		EnvDaemon+"=1",
		fmt.Sprintf("AURA_DIR=%s", dir),
		fmt.Sprintf("AURA_PORT=%d", port),
	)
	cmd.Stdout = logFile
	cmd.Stderr = logFile

	if err := cmd.Start(); err != nil {
		logFile.Close()
		return fmt.Errorf("fork daemon: %w", err)
	}

	// Detach: release the child so it outlives the CLI process.
	if err := cmd.Process.Release(); err != nil {
		return fmt.Errorf("release daemon process: %w", err)
	}

	logFile.Close()
	return nil
}

// RunDaemon is called inside the daemon process (AURA_DAEMON=1).
// It initialises the DB, starts the MCP server, writes the lock file, and blocks.
func RunDaemon(dir string, port int, sessionID string) error {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create aura dir: %w", err)
	}

	// Generate default config files if they don't exist.
	secret := generateLocalSecret()
	if err := WriteDefaultConfigs(dir, secret); err != nil {
		return fmt.Errorf("write configs: %w", err)
	}

	dbPath := filepath.Join(dir, "aura.db")
	database, err := db.Open(dbPath)
	if err != nil {
		return fmt.Errorf("open db: %w", err)
	}
	defer database.Close()

	if err := db.RunMigrations(database); err != nil {
		return fmt.Errorf("run migrations: %w", err)
	}

	mcp := &noopMCP{}
	if err := mcp.Start(port); err != nil {
		return fmt.Errorf("start mcp: %w", err)
	}
	defer mcp.Stop()

	if err := WriteLockFile(dir, os.Getpid(), mcp.Port()); err != nil {
		return fmt.Errorf("write lock file: %w", err)
	}
	defer RemoveLockFile(dir)

	// Set up session manager with auto-capture hook.
	sessMgr := session.New(database)
	store := memory.New(database)
	captureEngine := autocapture.NewCaptureEngine(store, autocapture.DefaultCaptureConfig())
	tracesDir := filepath.Join(dir, "traces")

	// Set up the wiki auto-learner — this is what makes Aura self-improving.
	// No IDE hooks needed. The daemon learns from everything automatically.
	wikiStore := wiki.NewStore(database)
	wikiEngine := wiki.NewEngine(wikiStore)
	autoLearner := wiki.NewAutoLearner(wikiEngine, store, captureEngine, database, dir)
	autoLearner.Start(wiki.DefaultAutoLearnConfig())
	defer autoLearner.Stop()

	sessMgr.SetOnEndHook(func(sessionID string) {
		go func() {
			// Auto-capture decisions into memory (existing behavior).
			transcriptPath := filepath.Join(tracesDir, sessionID+".jsonl")
			n, err := captureEngine.ProcessTranscript(sessionID, transcriptPath)
			if err != nil {
				slog.Warn("auto-capture failed", "session_id", sessionID, "err", err)
			} else {
				slog.Info("auto-capture completed", "session_id", sessionID, "captured", n)
			}

			// Auto-learn into wiki (new behavior).
			autoLearner.OnSessionEnd(sessionID)
		}()
	})

	d := &Daemon{dir: dir, db: database, mcp: mcp, port: mcp.Port(), sessID: sessionID}
	_ = d       // used by future tasks
	_ = sessMgr // used by future tasks

	// Block until a stop signal is received.
	waitForStop()
	return nil
}

// Stop signals a running daemon to shut down gracefully.
func Stop(dir string) error {
	info, err := ReadLockFile(dir)
	if err != nil {
		return fmt.Errorf("no lock file found (daemon not running?): %w", err)
	}
	if IsStale(info) {
		// Process is gone; clean up the stale lock file.
		_ = RemoveLockFile(dir)
		return fmt.Errorf("daemon not running (stale lock file removed)")
	}

	proc, err := os.FindProcess(info.PID)
	if err != nil {
		return fmt.Errorf("find process %d: %w", info.PID, err)
	}

	// SIGTERM gives the daemon a chance to flush and clean up.
	if err := proc.Signal(os.Interrupt); err != nil {
		return fmt.Errorf("signal daemon: %w", err)
	}
	return nil
}

// Status returns the current daemon status by reading the lock file and querying the DB.
func Status(dir string) (*StatusInfo, error) {
	info, err := ReadLockFile(dir)
	if err != nil {
		// No lock file means the daemon is not running.
		return &StatusInfo{Running: false}, nil
	}

	if IsStale(info) {
		_ = RemoveLockFile(dir)
		return &StatusInfo{Running: false}, nil
	}

	si := &StatusInfo{
		Running: true,
		PID:     info.PID,
		Port:    info.Port,
	}

	// Query memory entry count and latest session from the DB if it exists.
	dbPath := filepath.Join(dir, "aura.db")
	database, err := db.Open(dbPath)
	if err == nil {
		defer database.Close()
		si.MemoryCount = queryMemoryCount(database)
		si.SessionID = queryLatestSession(database)
	}

	return si, nil
}

// queryMemoryCount returns the total number of memory_entries rows.
func queryMemoryCount(database *sql.DB) int64 {
	var count int64
	_ = database.QueryRow("SELECT COUNT(*) FROM memory_entries").Scan(&count)
	return count
}

// queryLatestSession returns the most recently started active session ID.
func queryLatestSession(database *sql.DB) string {
	var id string
	_ = database.QueryRow(
		"SELECT id FROM sessions WHERE status='active' ORDER BY started_at DESC LIMIT 1",
	).Scan(&id)
	return id
}

// waitForStop blocks until SIGTERM or SIGINT is received.
func waitForStop() {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGTERM, syscall.SIGINT)
	<-ch
	signal.Stop(ch)
}
