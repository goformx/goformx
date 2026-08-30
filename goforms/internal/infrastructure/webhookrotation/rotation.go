// Package webhookrotation implements the privileged, offline maintenance boundary.
package webhookrotation

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/goformx/goforms/internal/domain/webhook"
)

const batchSize = 100

type Result struct {
	Endpoints   int64 `json:"endpoints"`
	Deliveries  int64 `json:"deliveries"`
	Reencrypted int64 `json:"reencrypted"`
}

// Run authenticates every retained ciphertext in one transaction, optionally
// re-encrypting it. Table locks block all competing writes (including enqueue,
// replay, endpoint edits and worker state changes), not just selected rows.
// Callers must stop every API/worker first: locks cannot retract an in-flight
// HTTP delivery or replace an old process's in-memory keyring.
func Run(ctx context.Context, connection *pgx.Conn, keyring *webhook.Cipher, verifyOnly bool) (Result, error) {
	if keyring.ActiveKeyID() == "" {
		return Result{}, errors.New("webhook rotation requires an explicit active keyring")
	}
	result := Result{}
	tx, err := connection.Begin(ctx)
	if err != nil {
		return Result{}, errors.New("cannot begin webhook rotation transaction")
	}
	defer func() {
		cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		_ = tx.Rollback(cleanup)
	}()
	// Do not wait indefinitely behind an operator who forgot maintenance mode.
	if _, err := tx.Exec(ctx, "SET LOCAL lock_timeout = '5s'"); err != nil {
		return Result{}, errors.New("cannot configure webhook rotation lock timeout")
	}
	if _, err := tx.Exec(ctx, "LOCK TABLE webhook_endpoints, webhook_deliveries IN SHARE ROW EXCLUSIVE MODE"); err != nil {
		return Result{}, errors.New("cannot lock webhook storage; stop all writers and retry")
	}
	for _, table := range []string{"webhook_endpoints", "webhook_deliveries"} {
		count, changed, err := rotateTable(ctx, tx, keyring, table, verifyOnly)
		if err != nil {
			return Result{}, err
		}
		if table == "webhook_endpoints" {
			result.Endpoints = count
		} else {
			result.Deliveries = count
		}
		result.Reencrypted += changed
	}
	if err := tx.Commit(ctx); err != nil {
		return Result{}, errors.New("webhook rotation commit unconfirmed; verify storage before resuming service")
	}
	return result, nil
}

type encryptedRow struct {
	id         string
	formID     string
	ciphertext []byte
}

func rotateTable(ctx context.Context, tx pgx.Tx, keyring *webhook.Cipher, table string, verifyOnly bool) (int64, int64, error) {
	// Identifiers originate only from the fixed allowlist in Run, never CLI input.
	identifier := pgx.Identifier{table}.Sanitize()
	var total, changed int64
	lastID := ""
	firstBatch := true
	for {
		rows, err := tx.Query(ctx, "SELECT uuid, form_id, encrypted_config FROM "+identifier+
			" WHERE $1 OR uuid > $2 ORDER BY uuid LIMIT $3", firstBatch, lastID, batchSize)
		if err != nil {
			return 0, 0, errors.New("cannot read webhook rotation batch")
		}
		batch, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (encryptedRow, error) {
			var record encryptedRow
			err := row.Scan(&record.id, &record.formID, &record.ciphertext)
			return record, err
		})
		if err != nil {
			return 0, 0, errors.New("cannot decode webhook rotation batch")
		}
		if len(batch) == 0 {
			return total, changed, nil
		}
		firstBatch = false
		for _, record := range batch {
			updated, needsRotation, err := keyring.Reencrypt(record.ciphertext, record.formID)
			if err != nil {
				return 0, 0, errors.New("webhook ciphertext authentication failed; transaction rolled back")
			}
			if verifyOnly && needsRotation {
				return 0, 0, errors.New("webhook storage still requires a previous or legacy key")
			}
			if needsRotation {
				if _, err := tx.Exec(ctx, "UPDATE "+identifier+" SET encrypted_config = $1 WHERE uuid = $2", updated, record.id); err != nil {
					return 0, 0, errors.New("cannot write webhook rotation batch; transaction rolled back")
				}
				changed++
			}
			total++
			lastID = record.id
		}
	}
}
