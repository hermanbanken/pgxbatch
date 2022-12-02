package pgxbatch_test

import (
	"context"
	"os/exec"

	"github.com/cockroachdb/cockroach-go/v2/testserver"
	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/pkg/errors"
)

func WithTestPool(fn func(*pgxpool.Pool) error, opts ...testserver.TestServerOpt) error {
	// Start
	server, err := start(opts...)
	if err != nil {
		return err
	}

	// Connect
	config, err := pgxpool.ParseConfig(server.PGURL().String())
	if err != nil {
		return errors.Wrap(err, "failed to connect to CRDB cluster")
	}
	pool, err := pgxpool.ConnectConfig(context.TODO(), config)
	if err != nil {
		return errors.Wrap(err, "failed to connect to CRDB cluster")
	}
	defer pool.Close()

	// Execute
	err = fn(pool)
	return err
}

func start(args ...testserver.TestServerOpt) (testserver.TestServer, error) {
	path, err := exec.LookPath("cockroach")
	if err == nil {
		args = append(args, testserver.CockroachBinaryPathOpt(path))
	}
	return testserver.NewTestServer(args...)
}

type testErrWithSQLState struct {
	code string
}

func (te testErrWithSQLState) SQLState() string {
	return te.code
}

func (te testErrWithSQLState) Error() string {
	return te.code
}

var RetryableError = testErrWithSQLState{"40001"}
