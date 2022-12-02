package pgxbatch_test

import (
	"context"
	"log"
	"testing"

	"github.com/cockroachdb/cockroach-go/v2/crdb/crdbpgx"
	"github.com/cockroachdb/cockroach-go/v2/testserver"
	"github.com/hermanbanken/pgxbatch/v4"
	"github.com/jackc/pgtype/pgxtype"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
	"go.uber.org/goleak"
)

var Pool *pgxpool.Pool
var exampleErr error

func ExampleBatch() {
	ctx := context.Background()
	b := &pgxbatch.Batch{}

	b.RegisterActor(func(q pgxtype.Querier) error {
		_, err := q.Exec(ctx, "DELETE FROM people")
		return err
	})
	b.RegisterActor(func(q pgxtype.Querier) (err error) {
		row := q.QueryRow(ctx, "INSERT INTO people (firstname, lastname) VALUES ('Joe', 'Cool') RETURNING id")
		var id int
		if err = row.Scan(&id); err == nil {
			log.Println("Inserted Joe with ID", id)
		}
		return
	})
	b.RegisterActor(func(q pgxtype.Querier) (err error) {
		row := q.QueryRow(ctx, "INSERT INTO people (firstname, lastname) VALUES ('Foo', 'Bar') RETURNING id")
		var id int
		if err = row.Scan(&id); err == nil {
			log.Println("Inserted Foo with ID", id)
		}
		return
	})
	b.RegisterActor(func(q pgxtype.Querier) error {
		rows, err := q.Query(ctx, "SELECT * FROM people")
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var first, last string
			var id int
			if err := rows.Scan(&first, &last, &id); err != nil {
				return err
			}
			log.Println(first, last, id)
		}
		return nil
	})

	exampleErr = b.Run(ctx, Pool)
}

func TestBatch(t *testing.T) {
	err := WithTestPool(func(p *pgxpool.Pool) error {
		opt := goleak.IgnoreCurrent()
		defer goleak.VerifyNone(t, opt)
		Pool = p

		Pool.Exec(context.TODO(), "CREATE TABLE people (firstname text, lastname text, id serial primary key)")
		defer Pool.Exec(context.TODO(), "DROP TABLE people")

		// Simple batch
		t.Run("batch", func(t *testing.T) {
			ExampleBatch()
		})

		// Cockroach Transaction with 1 retry
		t.Run("retried batch", func(t *testing.T) {
			iteration := 0
			crdbpgx.ExecuteTx(context.TODO(), Pool, pgx.TxOptions{IsoLevel: pgx.ReadUncommitted}, func(tx pgx.Tx) error {
				ExampleBatch()
				iteration++
				if iteration == 1 {
					return RetryableError
				}
				return exampleErr
			})
		})
		return exampleErr
	}, testserver.AddHttpPortOpt(8081))

	if err != nil {
		t.Error(err)
	}
}
