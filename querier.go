package pgxbatch

import (
	"context"

	"github.com/jackc/pgconn"
	"github.com/jackc/pgtype/pgxtype"
	"github.com/jackc/pgx/v4"
)

var _ pgxtype.Querier = &batchActorQuerier{}

type batchActorQuerier struct {
	Context context.Context
	Pause   chan<- batchCommand
	Resume  <-chan pgx.BatchResults
}

type batchCommand struct {
	Kind      batchCommandKind
	SQL       string
	arguments []interface{}
}

type batchCommandKind int

const (
	BatchExec     = batchCommandKind(0)
	BatchQuery    = batchCommandKind(1)
	BatchQueryRow = batchCommandKind(2)
)

// Exec implements pgxtype.Querier
func (b *batchActorQuerier) Exec(ctx context.Context, sql string, arguments ...interface{}) (pgconn.CommandTag, error) {
	select {
	case b.Pause <- batchCommand{BatchExec, sql, arguments}:
	case <-b.Context.Done():
		return nil, errCancelled{b.Context.Err()}
	}
	select {
	case res := <-b.Resume:
		return res.Exec()
	case <-b.Context.Done():
		return nil, errCancelled{b.Context.Err()}
	}
}

// Query implements pgxtype.Querier
func (b *batchActorQuerier) Query(ctx context.Context, sql string, optionsAndArgs ...interface{}) (pgx.Rows, error) {
	select {
	case b.Pause <- batchCommand{BatchExec, sql, optionsAndArgs}:
	case <-b.Context.Done():
		return nil, errCancelled{b.Context.Err()}
	}
	select {
	case res := <-b.Resume:
		return res.Query()
	case <-b.Context.Done():
		return nil, errCancelled{b.Context.Err()}
	}
}

// QueryRow implements pgxtype.Querier
func (b *batchActorQuerier) QueryRow(ctx context.Context, sql string, optionsAndArgs ...interface{}) pgx.Row {
	select {
	case b.Pause <- batchCommand{BatchQueryRow, sql, optionsAndArgs}:
	case <-b.Context.Done():
		return errRow{errCancelled{b.Context.Err()}}
	}
	select {
	case res := <-b.Resume:
		return res.QueryRow()
	case <-b.Context.Done():
		return errRow{errCancelled{b.Context.Err()}}
	}
}

type errCancelled struct{ error }

func (e errCancelled) Error() string {
	if e.error == nil {
		return "cancelled"
	}
	return e.error.Error()
}

type errRow struct{ error }

// Scan implements pgx.Row
func (r errRow) Scan(dest ...interface{}) error {
	return r.error
}

var _ pgx.Row = errRow{}
