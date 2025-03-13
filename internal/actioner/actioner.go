package actioner

import "github.com/cloudedugcp/responseEngine/internal/config"

type Actioner interface {
	Execute(ip string) error
	Name() string
}

func NewGCPFirewall(cfg config.ActionerConfig) *GCPFirewall {
	return &GCPFirewall{cfg: cfg}
}

func NewGCPStorage(cfg config.ActionerConfig) *GCPStorage {
	return &GCPStorage{cfg: cfg}
}
