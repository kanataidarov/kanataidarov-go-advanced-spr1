package postgres

import (
	"errors"
	"slices"

	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5/pgconn"
)

var retriableCodes = []string{
	pgerrcode.AdminShutdown,
	pgerrcode.CrashShutdown,
	pgerrcode.CannotConnectNow,
	pgerrcode.SerializationFailure,
	pgerrcode.DeadlockDetected,
}

func isRetriable(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgerrcode.IsConnectionException(pgErr.Code) || slices.Contains(retriableCodes, pgErr.Code)
	}

	var connectErr *pgconn.ConnectError
	if errors.As(err, &connectErr) {
		return true
	}

	return pgconn.SafeToRetry(err)
}
