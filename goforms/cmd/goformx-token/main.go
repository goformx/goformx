// Command goformx-token provisions and revokes server-side service tokens.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/goformx/goforms/internal/domain/auth"
)

var validScopes = map[auth.Scope]struct{}{
	auth.ScopeFormsRead: {}, auth.ScopeFormsWrite: {}, auth.ScopeFormsPublish: {}, auth.ScopeSubmissionsRead: {},
}

func main() {
	if err := run(context.Background(), os.Args[1:]); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, "goformx-token:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, arguments []string) error {
	if len(arguments) == 0 {
		return errors.New("usage: goformx-token <issue|revoke> [flags]")
	}
	switch arguments[0] {
	case "issue":
		return issue(ctx, arguments[1:])
	case "revoke":
		return revoke(ctx, arguments[1:])
	default:
		return fmt.Errorf("unknown command %q", arguments[0])
	}
}

func issue(ctx context.Context, arguments []string) error {
	flags := flag.NewFlagSet("issue", flag.ContinueOnError)
	owner := flags.String("owner", "", "owner UUID")
	scopeList := flags.String("scopes", "", "comma-separated scopes")
	ttl := flags.Duration("ttl", 24*time.Hour, "token lifetime")
	databaseURLFlag := flags.String("database-url", "", "PostgreSQL URL (defaults to DATABASE_URL)")
	if err := flags.Parse(arguments); err != nil {
		return err
	}
	databaseURL := resolveDatabaseURL(*databaseURLFlag)
	if databaseURL == "" {
		return errors.New("DATABASE_URL or --database-url is required")
	}
	scopes, err := parseScopes(*scopeList)
	if err != nil {
		return err
	}
	token, plaintext, err := auth.Issue(*owner, scopes, *ttl, time.Now().UTC())
	if err != nil {
		return err
	}
	connection, err := pgx.Connect(ctx, databaseURL)
	if err != nil {
		return fmt.Errorf("connect to PostgreSQL: %w", err)
	}
	defer func() { _ = connection.Close(ctx) }()
	encodedScopes, err := json.Marshal(scopeNames(scopes))
	if err != nil {
		return fmt.Errorf("encode scopes: %w", err)
	}
	_, err = connection.Exec(ctx, `
		INSERT INTO service_tokens (token_id, owner_id, token_hash, scopes, created_at, expires_at)
		VALUES ($1, $2, $3, $4::jsonb, $5, $6)
	`, token.ID, token.OwnerID, token.Hash[:], string(encodedScopes), token.CreatedAt, token.ExpiresAt)
	if err != nil {
		return fmt.Errorf("persist service token: %w", err)
	}
	return json.NewEncoder(os.Stdout).Encode(map[string]any{
		"token": plaintext, "tokenId": token.ID, "ownerId": token.OwnerID,
		"scopes": scopeNames(scopes), "expiresAt": token.ExpiresAt.Format(time.RFC3339),
	})
}

func revoke(ctx context.Context, arguments []string) error {
	flags := flag.NewFlagSet("revoke", flag.ContinueOnError)
	tokenID := flags.String("token-id", "", "non-secret token ID")
	databaseURLFlag := flags.String("database-url", "", "PostgreSQL URL (defaults to DATABASE_URL)")
	if err := flags.Parse(arguments); err != nil {
		return err
	}
	databaseURL := resolveDatabaseURL(*databaseURLFlag)
	if databaseURL == "" || *tokenID == "" {
		return errors.New("DATABASE_URL and --token-id are required")
	}
	connection, err := pgx.Connect(ctx, databaseURL)
	if err != nil {
		return fmt.Errorf("connect to PostgreSQL: %w", err)
	}
	defer func() { _ = connection.Close(ctx) }()
	result, err := connection.Exec(ctx, `UPDATE service_tokens SET revoked_at = now() WHERE token_id = $1 AND revoked_at IS NULL`, *tokenID)
	if err != nil {
		return fmt.Errorf("revoke service token: %w", err)
	}
	if result.RowsAffected() == 0 {
		return errors.New("active service token was not found")
	}
	return json.NewEncoder(os.Stdout).Encode(map[string]any{"tokenId": *tokenID, "revoked": true})
}

func resolveDatabaseURL(flagValue string) string {
	if flagValue != "" {
		return flagValue
	}
	return os.Getenv("DATABASE_URL")
}

func parseScopes(value string) ([]auth.Scope, error) {
	items := strings.Split(value, ",")
	seen := make(map[auth.Scope]struct{}, len(items))
	scopes := make([]auth.Scope, 0, len(items))
	for _, item := range items {
		scope := auth.Scope(strings.TrimSpace(item))
		if _, ok := validScopes[scope]; !ok {
			return nil, fmt.Errorf("unsupported scope %q", scope)
		}
		if _, duplicate := seen[scope]; duplicate {
			continue
		}
		seen[scope] = struct{}{}
		scopes = append(scopes, scope)
	}
	if len(scopes) == 0 {
		return nil, errors.New("at least one scope is required")
	}
	return scopes, nil
}

func scopeNames(scopes []auth.Scope) []string {
	names := make([]string, len(scopes))
	for index, scope := range scopes {
		names[index] = string(scope)
	}
	return names
}
