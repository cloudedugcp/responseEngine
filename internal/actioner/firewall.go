package actioner

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/cloudedugcp/responseEngine/internal/db" // Додаємо імпорт для db

	compute "cloud.google.com/go/compute/apiv1"
	computepb "google.golang.org/genproto/googleapis/cloud/compute/v1"
	"google.golang.org/protobuf/proto"
)

// FirewallActioner - діяч для Google Cloud Firewall
type FirewallActioner struct {
	projectID string
	timeout   time.Duration
	client    *compute.FirewallsClient
	db        *db.Database // Додаємо поле db
}

// NewFirewallActioner - створює новий FirewallActioner
func NewFirewallActioner(cfg ActionerConfig, database *db.Database) (*FirewallActioner, error) {
	fa := &FirewallActioner{
		projectID: cfg.Params["project_id"].(string),
		db:        database, // Ініціалізуємо БД
	}
	if timeout, ok := cfg.Params["timeout"].(int); ok {
		fa.timeout = time.Duration(timeout) * time.Minute
	} else {
		fa.timeout = 60 * time.Minute
	}

	var err error
	fa.client, err = compute.NewFirewallsRESTClient(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to create firewall client: %v", err)
	}
	return fa, nil
}

// Execute - виконує блокування IP
func (fa *FirewallActioner) Execute(event Event, params map[string]interface{}) error {
	if !fa.isIPBlocked(event.IP) {
		var priority int
		switch v := params["priority"].(type) {
		case int:
			priority = v
		case float64:
			priority = int(v)
		default:
			return fmt.Errorf("priority must be a number, got %T", v)
		}

		description := params["description"].(string)

		var timeout time.Duration
		if t, ok := params["timeout"].(string); ok {
			var err error
			timeout, err = time.ParseDuration(t)
			if err != nil {
				return fmt.Errorf("invalid timeout format: %v", err)
			}
		} else {
			timeout = fa.timeout
		}

		if err := fa.blockIP(event.IP, priority, description); err != nil {
			return fmt.Errorf("failed to block IP %s: %v", event.IP, err)
		}
		time.AfterFunc(timeout, func() {
			if err := fa.unblockIP(event.IP); err != nil {
				log.Printf("Failed to unblock IP %s: %v", event.IP, err)
			} else {
				log.Printf("Successfully unblocked IP %s after %s", event.IP, timeout)
				fa.db.LogAction(event.IP, "block", "unblocked", time.Now()) // Записуємо розблокування
			}
		})
	}
	return nil
}

// Name - повертає ім'я діяча
func (fa *FirewallActioner) Name() string { return "firewall" }

// isIPBlocked - перевіряє, чи IP уже заблоковано
func (fa *FirewallActioner) isIPBlocked(ip string) bool {
	req := &computepb.ListFirewallsRequest{Project: fa.projectID}
	it := fa.client.List(context.Background(), req)
	for firewall, err := it.Next(); err == nil; firewall, err = it.Next() {
		for _, rule := range firewall.SourceRanges {
			if rule == ip+"/32" {
				return true
			}
		}
	}
	return false
}

// blockIP - блокує IP у GCP Firewall
func (fa *FirewallActioner) blockIP(ip string, priority int, description string) error {
	safeIP := strings.ReplaceAll(ip, ".", "-")
	ruleName := fmt.Sprintf("block-%s-%d", safeIP, time.Now().UnixNano())
	if len(ruleName) > 63 {
		ruleName = ruleName[:63]
		ruleName = strings.TrimRight(ruleName, "-")
	}

	rule := &computepb.Firewall{
		Name:         proto.String(ruleName),
		Description:  &description,
		Direction:    proto.String("INGRESS"),
		Priority:     proto.Int32(int32(priority)),
		SourceRanges: []string{ip + "/32"},
		Denied:       []*computepb.Denied{{IPProtocol: proto.String("all")}},
	}
	req := &computepb.InsertFirewallRequest{
		Project:          fa.projectID,
		FirewallResource: rule,
	}
	op, err := fa.client.Insert(context.Background(), req)
	if err != nil {
		log.Printf("Failed to block IP %s: %v", ip, err)
		return err
	}
	if err := op.Wait(context.Background()); err != nil {
		log.Printf("Failed to wait for firewall insertion: %v", err)
		return err
	}
	return nil
}

// unblockIP - розблокує IP
func (fa *FirewallActioner) unblockIP(ip string) error {
	safeIP := strings.ReplaceAll(ip, ".", "-")
	req := &computepb.ListFirewallsRequest{Project: fa.projectID}
	it := fa.client.List(context.Background(), req)
	for firewall, err := it.Next(); err == nil; firewall, err = it.Next() {
		if strings.Contains(*firewall.Name, safeIP) {
			delReq := &computepb.DeleteFirewallRequest{
				Project:  fa.projectID,
				Firewall: *firewall.Name,
			}
			op, err := fa.client.Delete(context.Background(), delReq)
			if err != nil {
				return err
			}
			return op.Wait(context.Background())
		}
	}
	return nil
}
