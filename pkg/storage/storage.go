package storage

import (
	"github.com/arqut/arqut-edge-ce/pkg/storage/repositories"
	"gorm.io/gorm"
)

// Storage is the database storage interface
type Storage interface {
	DB() *gorm.DB
	ServiceRepo() *repositories.ServiceRepository
	Close() error
}
