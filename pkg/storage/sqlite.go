package storage

import (
	"fmt"
	"log"

	"github.com/arqut/arqut-edge-ce/pkg/logger"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// SQLiteStorage implements Storage interface using SQLite
type SQLiteStorage struct {
	db     *gorm.DB
	logger *logger.Logger
}

// NewSQLiteStorage creates a new SQLite storage instance
func NewSQLiteStorage(dbPath string, appLogger *logger.Logger) (Storage, error) {
	gormLogger := gormlogger.Default.LogMode(gormlogger.Silent)

	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{
		Logger: gormLogger,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	if appLogger != nil {
		appLogger.Info("SQLite database opened: %s", dbPath)
	} else {
		log.Printf("SQLite database opened: %s", dbPath)
	}

	return &SQLiteStorage{
		db:     db,
		logger: appLogger,
	}, nil
}

// DB returns the underlying GORM database instance
func (s *SQLiteStorage) DB() *gorm.DB {
	return s.db
}

// Close closes the database connection
func (s *SQLiteStorage) Close() error {
	sqlDB, err := s.db.DB()
	if err != nil {
		return fmt.Errorf("failed to get database instance: %w", err)
	}

	if err := sqlDB.Close(); err != nil {
		return fmt.Errorf("failed to close database: %w", err)
	}

	if s.logger != nil {
		s.logger.Info("SQLite database closed")
	} else {
		log.Println("SQLite database closed")
	}
	return nil
}
