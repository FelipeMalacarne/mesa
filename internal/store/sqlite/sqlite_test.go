package sqlite_test

import (
	"context"
	"database/sql"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/felipemalacarne/mesa/internal/core/connection"
	"github.com/felipemalacarne/mesa/internal/store"
	"github.com/felipemalacarne/mesa/internal/store/sqlite"
	"github.com/felipemalacarne/mesa/internal/store/storetest"
)

func TestStore(t *testing.T) {
	storetest.Run(t, func(t *testing.T) store.Store {
		st, err := sqlite.Open(context.Background(), sqlite.Memory)
		if err != nil {
			t.Fatalf("open: %v", err)
		}
		t.Cleanup(func() { _ = st.Close() })
		return st
	})
}

// TestOpenFile covers what the in-memory suite cannot: parent directories,
// data surviving a reopen, migrations recorded once and WAL mode on disk.
func TestOpenFile(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "nested", "config", "mesa.db")

	st, err := sqlite.Open(ctx, path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	conn, err := connection.New(connection.Input{
		Name: "local", Engine: "postgres", Host: "localhost", User: "mesa",
	}, time.Now().UTC())
	if err != nil {
		t.Fatal(err)
	}
	if err := st.Connections().Put(ctx, conn); err != nil {
		t.Fatalf("put: %v", err)
	}
	if err := st.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	st, err = sqlite.Open(ctx, path)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer func() { _ = st.Close() }()
	got, err := st.Connections().Get(ctx, conn.ID)
	if err != nil {
		t.Fatalf("get after reopen: %v", err)
	}
	if got.Name != conn.Name {
		t.Fatalf("name after reopen = %q, want %q", got.Name, conn.Name)
	}

	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	var applied int
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM mesa_migrations`).Scan(&applied); err != nil {
		t.Fatal(err)
	}
	if applied != 1 {
		t.Fatalf("migrations applied = %d, want 1", applied)
	}
	var mode string
	if err := db.QueryRowContext(ctx, `PRAGMA journal_mode`).Scan(&mode); err != nil {
		t.Fatal(err)
	}
	if mode != "wal" {
		t.Fatalf("journal_mode = %q, want wal", mode)
	}
}

func TestOpenEmptyPath(t *testing.T) {
	if _, err := sqlite.Open(context.Background(), ""); err == nil {
		t.Fatal("open with an empty path succeeded")
	}
}

// TestConcurrentWriters checks the pool settings on a file: several
// goroutines writing at once queue on the busy timeout instead of failing.
func TestConcurrentWriters(t *testing.T) {
	ctx := context.Background()
	st, err := sqlite.Open(ctx, filepath.Join(t.TempDir(), "mesa.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer func() { _ = st.Close() }()

	const writers, perWriter = 8, 25
	conn := uuid.New()
	var wg sync.WaitGroup
	failures := make(chan error, writers*perWriter)
	for range writers {
		wg.Go(func() {
			for i := range perWriter {
				err := st.History().Add(ctx, store.QueryRun{
					ID: uuid.New(), ConnectionID: conn, Database: "app", SQL: "select 1",
					RanAt: time.Now().Add(time.Duration(i) * time.Millisecond),
				})
				if err != nil {
					failures <- err
				}
			}
		})
	}
	wg.Wait()
	close(failures)
	for err := range failures {
		t.Errorf("concurrent Add: %v", err)
	}
	got, err := st.History().List(ctx, conn, writers*perWriter+1)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(got) != writers*perWriter {
		t.Fatalf("stored %d runs, want %d", len(got), writers*perWriter)
	}
}
