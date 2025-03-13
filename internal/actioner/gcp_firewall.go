package actioner

import (
	"context"
	"log"
	"strings"

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

	ruleName := "block-" + strings.ReplaceAll(ip, ".", "-")
	ruleName = strings.ToLower(ruleName)

	firewall := &compute.Firewall{
		Name:         ruleName,
		SourceRanges: []string{ip + "/32"},
		Denied:       []*compute.FirewallDenied{{IPProtocol: "all"}},
	}

	_, err = svc.Firewalls.Insert(g.cfg.ProjectID, firewall).Do()
	if err != nil {
		log.Printf("Failed to block IP %s: %v", ip, err)
		return err
	}
	log.Printf("Successfully blocked IP %s with rule %s", ip, ruleName)
	return nil
}

func (g *GCPFirewall) Unblock(ip string) error {
	ctx := context.Background()
	svc, err := compute.NewService(ctx, option.WithCredentialsFile(g.cfg.CredentialsFile))
	if err != nil {
		return err
	}

	ruleName := "block-" + strings.ReplaceAll(ip, ".", "-")
	ruleName = strings.ToLower(ruleName)

	if _, err := svc.Firewalls.Delete(g.cfg.ProjectID, ruleName).Do(); err != nil {
		log.Printf("Failed to unblock IP %s: %v", ip, err)
		return err
	}
	log.Printf("Successfully unblocked IP %s by deleting rule %s", ip, ruleName)
	return nil
}
