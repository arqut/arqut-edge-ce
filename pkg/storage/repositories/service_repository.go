package repositories

import (
	"fmt"

	"github.com/arqut/arqut-edge-ce/pkg/models"
	"github.com/arqut/arqut-edge-ce/pkg/utils"
	"gorm.io/gorm"
)

type ServiceRepository struct {
	db *gorm.DB
}

func NewServiceRepository(db *gorm.DB) *ServiceRepository {
	db.AutoMigrate(&models.ProxyService{})
	return &ServiceRepository{db: db}
}

// AddService creates a new proxy service
func (r *ServiceRepository) AddService(name, localHost string, localPort int, tunnelPort int, protocol string) (*models.ProxyService, error) {
	// Validate protocol
	if protocol != "http" && protocol != "websocket" {
		return nil, fmt.Errorf("unsupported protocol: %s (supported: http, websocket)", protocol)
	}

	// Validate input
	if localPort < 1 || localPort > 65535 {
		return nil, fmt.Errorf("invalid local port: %d", localPort)
	}
	if localHost == "" {
		return nil, fmt.Errorf("local host cannot be empty")
	}
	if name == "" {
		return nil, fmt.Errorf("service name cannot be empty")
	}

	serviceID, _ := utils.GenerateID()
	service := &models.ProxyService{
		ID:         serviceID,
		Name:       name,
		TunnelPort: tunnelPort,
		LocalHost:  localHost,
		LocalPort:  localPort,
		Protocol:   protocol,
		Enabled:    true,
	}

	if err := r.db.Create(service).Error; err != nil {
		return nil, err
	}

	return service, nil
}

// UpdateService updates a proxy service
func (r *ServiceRepository) UpdateService(id string, config models.ProxyServiceConfig) error {
	updates := map[string]any{}

	if config.Name != nil {
		if *config.Name == "" {
			return fmt.Errorf("service name cannot be empty")
		}
		updates["name"] = *config.Name
	}
	if config.LocalHost != nil {
		if *config.LocalHost == "" {
			return fmt.Errorf("local host cannot be empty")
		}
		updates["local_host"] = *config.LocalHost
	}
	if config.LocalPort != nil {
		if *config.LocalPort < 1 || *config.LocalPort > 65535 {
			return fmt.Errorf("invalid local port: %d", *config.LocalPort)
		}
		updates["local_port"] = *config.LocalPort
	}
	if config.Enabled != nil {
		updates["enabled"] = *config.Enabled
	}

	if len(updates) == 0 {
		return fmt.Errorf("no fields to update")
	}

	if err := r.db.Model(&models.ProxyService{}).Where("id = ?", id).Updates(updates).Error; err != nil {
		return err
	}

	return nil
}

// DeleteService deletes a proxy service
func (r *ServiceRepository) DeleteService(id string) error {
	if err := r.db.Where("id = ?", id).Delete(&models.ProxyService{}).Error; err != nil {
		return err
	}
	return nil
}

// GetServices returns all proxy services
func (r *ServiceRepository) GetServices() ([]*models.ProxyService, error) {
	var services []*models.ProxyService
	if err := r.db.Order("name").Find(&services).Error; err != nil {
		return nil, err
	}
	return services, nil
}

// GetService returns a single proxy service by ID
func (r *ServiceRepository) GetService(id string) (*models.ProxyService, error) {
	var service models.ProxyService
	if err := r.db.Where("id = ?", id).First(&service).Error; err != nil {
		return nil, err
	}
	return &service, nil
}

// GetServiceByHostPort finds a service by host and port
func (r *ServiceRepository) GetServiceByHostPort(host string, port int) (*models.ProxyService, error) {
	var service models.ProxyService
	if err := r.db.Where("local_host = ? AND local_port = ?", host, port).First(&service).Error; err != nil {
		return nil, err
	}
	return &service, nil
}

// GetUsedPorts returns a list of all used tunnel ports
func (r *ServiceRepository) GetUsedPorts() ([]int, error) {
	var usedPorts []int
	if err := r.db.Model(&models.ProxyService{}).Pluck("tunnel_port", &usedPorts).Error; err != nil {
		return nil, err
	}
	return usedPorts, nil
}

func (r *ServiceRepository) Count() (int, error) {
	var count int64
	if err := r.db.Model(&models.ProxyService{}).Count(&count).Error; err != nil {
		return 0, err
	}
	return int(count), nil
}

// Clear removes all proxy services
func (r *ServiceRepository) Clear() error {
	return r.db.Delete(&models.ProxyService{}, "1=1").Error
}
