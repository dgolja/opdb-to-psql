package pgsql

import (
	"context"
	"log/slog"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// loggingDB wraps a DB and logs every statement at Debug level.
type loggingDB struct {
	DB
	log *slog.Logger
}

func (l *loggingDB) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	l.log.DebugContext(ctx, "exec", "sql", sql, "args", args)
	return l.DB.Exec(ctx, sql, args...)
}

func (l *loggingDB) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	l.log.DebugContext(ctx, "query", "sql", sql, "args", args)
	return l.DB.Query(ctx, sql, args...)
}

// Begin returns a transaction that logs its statements the same way.
func (l *loggingDB) Begin(ctx context.Context) (pgx.Tx, error) {
	tx, err := l.DB.Begin(ctx)
	if err != nil {
		return nil, err
	}
	return &loggingTx{Tx: tx, log: l.log}, nil
}

// loggingTx wraps a pgx.Tx and logs every statement at Debug level.
type loggingTx struct {
	pgx.Tx
	log *slog.Logger
}

func (l *loggingTx) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	l.log.DebugContext(ctx, "exec", "sql", sql, "args", args)
	return l.Tx.Exec(ctx, sql, args...)
}

func (l *loggingTx) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	l.log.DebugContext(ctx, "query", "sql", sql, "args", args)
	return l.Tx.Query(ctx, sql, args...)
}
