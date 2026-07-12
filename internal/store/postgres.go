package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/url"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"

	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/jackc/pgx/v5/stdlib"
)

// PostgresClient implements [Client] using PostgreSQL.
type PostgresClient struct {
	db *sql.DB
}

// NewPostgresClient returns a new PostgreClient instance.
func NewPostgresClient(
	ctx context.Context, addr, username, password, databaseName string,
) (*PostgresClient, error) {
	// Form the DSN safely.
	dsn := &url.URL{
		Scheme:   "postgresql",
		User:     url.UserPassword(username, password),
		Host:     addr,
		Path:     databaseName,
		RawQuery: "sslmode=disable",
	}

	// Connect to database.
	db, err := sql.Open("pgx", dsn.String())
	if err != nil {
		return nil, fmt.Errorf("failed to connect database: %w", err)
	}

	// Verify connection.
	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &PostgresClient{db: db}, nil
}

// RunMigrations uses golang-migrate to run migrations. The path should point to the directory
// containing the migrations files in the format that golang-migrate accepts.
func (p *PostgresClient) RunMigrations(ctx context.Context, path string) error {
	// Use existing database client for migrations.
	driver, err := postgres.WithInstance(p.db, &postgres.Config{})
	if err != nil {
		return fmt.Errorf("failed to create postgres driver for migrations: %w", err)
	}

	// Create a client to execute migrations.
	migrationClient, err := migrate.NewWithDatabaseInstance(path, "postgres", driver)
	if err != nil {
		return fmt.Errorf("failed to create migration client: %w", err)
	}

	// Note: Do not close the migration client here as it will close the *sql.DB connection.

	// Run migrations.
	if err := migrationClient.Up(); err != nil {
		if errors.Is(err, migrate.ErrNoChange) {
			slog.InfoContext(ctx, "no database migrations to run")
			return nil
		}
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	return nil
}

// Close the database connection.
func (p *PostgresClient) Close(_ context.Context) error {
	return p.db.Close()
}
