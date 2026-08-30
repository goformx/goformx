// Command goformx-webhook-keys rotates encrypted webhook storage during maintenance.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/goformx/goforms/internal/infrastructure/config"
	"github.com/goformx/goforms/internal/infrastructure/webhookrotation"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, os.Args[1:], os.Stdout); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, "goformx-webhook-keys:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string, output io.Writer) error {
	if len(args) != 1 || (args[0] != "rotate" && args[0] != "verify") {
		return errors.New("usage: goformx-webhook-keys <rotate|verify>; configure database and keys through environment only")
	}
	configuration := config.WebhookConfig{
		EncryptionKey:         os.Getenv("WEBHOOK_ENCRYPTION_KEY"),
		EncryptionKeyring:     os.Getenv("WEBHOOK_ENCRYPTION_KEYRING"),
		ActiveEncryptionKeyID: os.Getenv("WEBHOOK_ACTIVE_ENCRYPTION_KEY_ID"),
	}
	if configuration.ActiveEncryptionKeyID == "" {
		return errors.New("WEBHOOK_ACTIVE_ENCRYPTION_KEY_ID is required")
	}
	keyring, err := configuration.Cipher()
	if err != nil {
		return err
	}
	if os.Getenv("DATABASE_URL") == "" {
		return errors.New("DATABASE_URL is required")
	}
	// Deliberately discard pgx parse/connect/server errors: they may contain a
	// DSN, passwords or attacker-controlled database diagnostics.
	connection, err := pgx.Connect(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		return errors.New("cannot connect to webhook database")
	}
	defer func() {
		cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
		defer cancel()
		_ = connection.Close(cleanup)
	}()
	result, err := webhookrotation.Run(ctx, connection, keyring, args[0] == "verify")
	if err != nil {
		return err
	}
	if err := json.NewEncoder(output).Encode(result); err != nil {
		return errors.New("operation committed but summary output failed; run verify before resuming service")
	}
	return nil
}
