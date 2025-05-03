package database

import (
	"context"
	"fmt"
	"log"
	"gomodules.avm99963.com/zenithplanner/internal/config"

	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository holds the database connection pool.
type Repository struct {
	pool *pgxpool.Pool
}

// NewDBPool creates a new PostgreSQL connection pool.
func NewDBPool(ctx context.Context, dbCfg config.DBConfig) (*pgxpool.Pool, error) {
	poolConfig, err := pgxpool.ParseConfig(dbCfg.ConnectionString)
	if err != nil {
		return nil, fmt.Errorf("unable to parse database connection string: %w", err)
	}

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("unable to create connection pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("unable to ping database: %w", err)
	}

	log.Println("Successfully connected to PostgreSQL database.")
	return pool, nil
}

// NewRepository creates a new repository with the given connection pool.
func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

// Close closes the database connection pool.
func (r *Repository) Close() {
	r.pool.Close()
}
