package models

import "time"

// ProxyService represents a proxy service configuration
type ProxyService struct {
	ID         string    `json:"id" gorm:"type:varchar(8);primaryKey"`
	Name       string    `json:"name" gorm:"type:varchar(128)"`
	TunnelPort int       `json:"tunnel_port"`
	LocalHost  string    `json:"local_host"`
	LocalPort  int       `json:"local_port"`
	Path       *string   `json:"path,omitempty" gorm:"type:varchar(255)"` // Optional path prefix (e.g., "/api")
	Protocol   string    `json:"protocol" gorm:"type:varchar(10)"`        // "http" or "websocket"
	Enabled    bool      `json:"enabled"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// TableName overrides the table name
func (ProxyService) TableName() string {
	return "proxy_services"
}

// ProxyServiceConfig represents partial update configuration
type ProxyServiceConfig struct {
	Name      *string `json:"name,omitempty"`
	LocalHost *string `json:"local_host,omitempty"`
	LocalPort *int    `json:"local_port,omitempty"`
	Path      *string `json:"path,omitempty"`
	Enabled   *bool   `json:"enabled,omitempty"`
}
