package pgxbatch

import (
	"context"

	"github.com/jackc/pgtype/pgxtype"
	"github.com/jackc/pgx/v4"
)

// Implemented by pgxpool.Pool and pgx.Conn and pgx.Transaction
type batchRunner interface {
	SendBatch(ctx context.Context, b *pgx.Batch) pgx.BatchResults
}

type Batch struct {
	batch  pgx.Batch
	actors []func(pgxtype.Querier) error
}

func (b *Batch) Run(ctx context.Context, runner batchRunner) error {
	var handlers []func(pgx.BatchResults) error
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	for _, actor := range b.actors {
		// Wait for each actor to execute a Exec/Query/QueryRow
		pause := make(chan batchCommand)
		resume := make(chan pgx.BatchResults)
		complete := make(chan error)
		go func() {
			select {
			case complete <- actor(&batchActorQuerier{
				Context: ctx,
				Pause:   pause,
				Resume:  resume,
			}):
			case <-ctx.Done():
			}
		}()
		cmd := <-pause

		// Setup the resumption handler
		b.batch.Queue(cmd.SQL, cmd.arguments...)
		handlers = append(handlers, func(br pgx.BatchResults) error {
			resume <- br
			return <-complete
		})
	}

	// Send the batch & then iterate over all registered handlers
	if b.batch.Len() == 0 {
		return nil
	}
	batchResults := runner.SendBatch(ctx, &b.batch)
	defer batchResults.Close()
	for _, fn := range handlers {
		err := fn(batchResults)
		if err != nil {
			return err
		}
	}
	return nil
}

func (b *Batch) RegisterActor(actor func(pgxtype.Querier) error) {
	b.actors = append(b.actors, actor)
}
