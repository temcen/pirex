package database

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
	"github.com/redis/go-redis/v9"
	"github.com/sirupsen/logrus"

	"github.com/temcen/pirex/internal/config"
)

type Database struct {
	PG     *pgxpool.Pool
	Neo4j  neo4j.DriverWithContext
	Redis  *RedisClients
	logger *logrus.Logger
}

type RedisClients struct {
	Hot  *redis.Client
	Warm *redis.Client
	Cold *redis.Client
}

func New(cfg *config.Config, logger *logrus.Logger) (*Database, error) {
	db := &Database{
		logger: logger,
	}

	// Initialize PostgreSQL
	if err := db.initPostgreSQL(cfg); err != nil {
		return nil, fmt.Errorf("failed to initialize PostgreSQL: %w", err)
	}

	// Initialize Neo4j
	if err := db.initNeo4j(cfg); err != nil {
		return nil, fmt.Errorf("failed to initialize Neo4j: %w", err)
	}

	// Initialize Redis clients
	if err := db.initRedis(cfg); err != nil {
		return nil, fmt.Errorf("failed to initialize Redis: %w", err)
	}

	return db, nil
}

func (db *Database) initPostgreSQL(cfg *config.Config) error {
	config, err := pgxpool.ParseConfig(cfg.Database.URL)
	if err != nil {
		return fmt.Errorf("failed to parse PostgreSQL config: %w", err)
	}

	// Configure connection pool
	config.MaxConns = int32(cfg.Database.MaxConnections)
	config.MaxConnIdleTime = cfg.Database.MaxIdleTime
	config.MaxConnLifetime = cfg.Database.MaxLifetime
	config.ConnConfig.ConnectTimeout = cfg.Database.ConnectTimeout

	pool, err := pgxpool.NewWithConfig(context.Background(), config)
	if err != nil {
		return fmt.Errorf("failed to create PostgreSQL pool: %w", err)
	}

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := pool.Ping(ctx); err != nil {
		return fmt.Errorf("failed to ping PostgreSQL: %w", err)
	}

	db.PG = pool
	db.logger.Info("PostgreSQL connection established")
	return nil
}

func (db *Database) initNeo4j(cfg *config.Config) error {
	driver, err := neo4j.NewDriverWithContext(
		cfg.Neo4j.URL,
		neo4j.BasicAuth(cfg.Neo4j.Username, cfg.Neo4j.Password, ""),
		func(config *neo4j.Config) {
			config.MaxConnectionPoolSize = 10
			config.ConnectionAcquisitionTimeout = 30 * time.Second
		},
	)
	if err != nil {
		return fmt.Errorf("failed to create Neo4j driver: %w", err)
	}

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := driver.VerifyConnectivity(ctx); err != nil {
		return fmt.Errorf("failed to verify Neo4j connectivity: %w", err)
	}

	db.Neo4j = driver
	db.logger.Info("Neo4j connection established")
	return nil
}

func (db *Database) initRedis(cfg *config.Config) error {
	db.Redis = &RedisClients{}

	// Initialize Hot Redis (user sessions, rate limiting)
	db.Redis.Hot = redis.NewClient(&redis.Options{
		Addr:         cfg.Redis.Hot.URL,
		MaxRetries:   cfg.Redis.Hot.MaxRetries,
		PoolSize:     cfg.Redis.Hot.PoolSize,
		ReadTimeout:  cfg.Redis.Hot.Timeout,
		WriteTimeout: cfg.Redis.Hot.Timeout,
	})

	// Initialize Warm Redis (recommendations, metadata)
	db.Redis.Warm = redis.NewClient(&redis.Options{
		Addr:         cfg.Redis.Warm.URL,
		MaxRetries:   cfg.Redis.Warm.MaxRetries,
		PoolSize:     cfg.Redis.Warm.PoolSize,
		ReadTimeout:  cfg.Redis.Warm.Timeout,
		WriteTimeout: cfg.Redis.Warm.Timeout,
	})

	// Initialize Cold Redis (embeddings, long-term data)
	db.Redis.Cold = redis.NewClient(&redis.Options{
		Addr:         cfg.Redis.Cold.URL,
		MaxRetries:   cfg.Redis.Cold.MaxRetries,
		PoolSize:     cfg.Redis.Cold.PoolSize,
		ReadTimeout:  cfg.Redis.Cold.Timeout,
		WriteTimeout: cfg.Redis.Cold.Timeout,
	})

	// Test connections
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := db.Redis.Hot.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("failed to ping Redis Hot: %w", err)
	}

	if err := db.Redis.Warm.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("failed to ping Redis Warm: %w", err)
	}

	if err := db.Redis.Cold.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("failed to ping Redis Cold: %w", err)
	}

	db.logger.Info("Redis connections established")
	return nil
}

func (db *Database) Close() error {
	var errors []error

	// Close PostgreSQL
	if db.PG != nil {
		db.PG.Close()
		db.logger.Info("PostgreSQL connection closed")
	}

	// Close Neo4j
	if db.Neo4j != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := db.Neo4j.Close(ctx); err != nil {
			errors = append(errors, fmt.Errorf("failed to close Neo4j: %w", err))
		} else {
			db.logger.Info("Neo4j connection closed")
		}
	}

	// Close Redis connections
	if db.Redis != nil {
		if db.Redis.Hot != nil {
			if err := db.Redis.Hot.Close(); err != nil {
				errors = append(errors, fmt.Errorf("failed to close Redis Hot: %w", err))
			}
		}
		if db.Redis.Warm != nil {
			if err := db.Redis.Warm.Close(); err != nil {
				errors = append(errors, fmt.Errorf("failed to close Redis Warm: %w", err))
			}
		}
		if db.Redis.Cold != nil {
			if err := db.Redis.Cold.Close(); err != nil {
				errors = append(errors, fmt.Errorf("failed to close Redis Cold: %w", err))
			}
		}
		if len(errors) == 0 {
			db.logger.Info("Redis connections closed")
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("errors closing database connections: %v", errors)
	}

	return nil
}
