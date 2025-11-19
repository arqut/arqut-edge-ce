package storage

import (
	"gorm.io/gorm"
)

// Storage is the database storage interface
type Storage interface {
	// DB returns the underlying GORM database instance
	DB() *gorm.DB

	Close() error
}
