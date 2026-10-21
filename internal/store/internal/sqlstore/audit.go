package sqlstore

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/felipemalacarne/mesa/internal/core/errs"
	"github.com/felipemalacarne/mesa/internal/store"
)

const auditColumns = `id, connection_id, database, environment, actor, surface, summary, sql, down_sql, status, error, started_at, finished_at`

type audit struct{ *Store }

func (a audit) Begin(ctx context.Context, e store.AuditEntry) error {
	if e.Status == "" {
		e.Status = store.AuditStarted
	}
	err := a.exec(ctx, `
		INSERT INTO audit (`+auditColumns+`)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		e.ID, e.ConnectionID, e.Database, string(e.Environment), e.Actor, e.Surface,
		e.Summary, e.SQL, e.DownSQL, string(e.Status), e.Error,
		a.dialect.Time(e.StartedAt), a.nullTime(e.FinishedAt))
	if err != nil {
		return fail("begin audit entry", err)
	}
	return nil
}

func (a audit) Finish(ctx context.Context, id uuid.UUID, status store.AuditStatus, errMsg string, at time.Time) error {
	ok, err := a.affected(ctx, `UPDATE audit SET status = ?, error = ?, finished_at = ? WHERE id = ?`,
		string(status), errMsg, a.dialect.Time(at), id)
	if err != nil {
		return fail("finish audit entry", err)
	}
	if !ok {
		return errs.Newf(errs.NotFound, "audit entry %s not found", id)
	}
	return nil
}

// List returns the newest entries first, all connections when connectionID is nil.
func (a audit) List(ctx context.Context, connectionID *uuid.UUID, limit int) ([]store.AuditEntry, error) {
	query := `SELECT ` + auditColumns + ` FROM audit`
	var args []any
	if connectionID != nil {
		query += ` WHERE connection_id = ?`
		args = append(args, *connectionID)
	}
	query += ` ORDER BY started_at DESC, id DESC LIMIT ?`
	args = append(args, limitOrDefault(limit))
	out, err := selectAll(ctx, a.Store, scanAuditEntry, query, args...)
	if err != nil {
		return nil, fail("list audit entries", err)
	}
	return out, nil
}

func scanAuditEntry(sc scanner) (store.AuditEntry, error) {
	var e store.AuditEntry
	var started, finished scanTime
	err := sc.Scan(&e.ID, &e.ConnectionID, &e.Database, &e.Environment, &e.Actor, &e.Surface,
		&e.Summary, &e.SQL, &e.DownSQL, &e.Status, &e.Error, &started, &finished)
	e.StartedAt, e.FinishedAt = started.t, finished.ptr()
	return e, err
}
