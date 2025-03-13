package actioner

import (
	"context"
	"fmt"
	"log"

	"github.com/cloudedugcp/responseEngine/internal/config"

	"google.golang.org/api/compute/v1"
	"google.golang.org/api/option"
)

type GCPFirewall struct {
	cfg config.ActionerConfig
}

func (g *GCPFirewall) Name() string {
	return "gcp_firewall"
}

func (g *GCPFirewall) Execute(ip string) error {
	ctx := context.Background()
	svc, err := compute.NewService(ctx, option.WithCredentialsFile(g.cfg.CredentialsFile))
	if err != nil {
		return err
	}

	ruleName := fmt.Sprintf("block-%s", ip)
	firewall := &compute.Firewall{
		Name:         ruleName,
		SourceRanges: []string{ip + "/32"},
		Denied:       []*compute.FirewallDenied{{IPProtocol: "all"}},
	}

	_, err = svc.Firewalls.Insert(g.cfg.ProjectID, firewall).Do()
	if err != nil {
		log.Printf("Failed to block IP %s: %v", ip, err)
	}
	return err
}
