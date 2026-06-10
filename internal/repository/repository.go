// Package repository is the persistence layer over SQLite/GORM.
package repository

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/ncruces/go-sqlite3/gormlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"

	"github.com/jwhumphries/bandwidth/internal/model"
)

// Repo provides all database operations.
type Repo struct {
	db *gorm.DB
}

// Open opens (creating if needed) the SQLite database at path, enables WAL,
// and migrates the schema. Use ":memory:" for tests.
func Open(path string) (*Repo, error) {
	if path != ":memory:" {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return nil, fmt.Errorf("create database directory: %w", err)
		}
	}
	dsn := fmt.Sprintf(
		"file:%s?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)",
		path,
	)
	db, err := gorm.Open(gormlite.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	if err := db.AutoMigrate(
		&model.User{},
		&model.Session{},
		&model.BackupCode{},
		&model.PasswordReset{},
	); err != nil {
		return nil, fmt.Errorf("migrate database: %w", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("access connection pool: %w", err)
	}
	if path == ":memory:" {
		// A second pool connection would see a separate, empty in-memory DB.
		sqlDB.SetMaxOpenConns(1)
	}
	return &Repo{db: db}, nil
}

// Close releases the underlying database connections.
func (r *Repo) Close() error {
	sqlDB, err := r.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}
