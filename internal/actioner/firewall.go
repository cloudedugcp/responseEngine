package actioner

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	compute "cloud.google.com/go/compute/apiv1"
	computepb "google.golang.org/genproto/googleapis/cloud/compute/v1"
	"google.golang.org/protobuf/proto"
)

// FirewallActioner - діяч для Google Cloud Firewall
type FirewallActioner struct {
	projectID string
	timeout   time.Duration
	client    *compute.FirewallsClient
}

// NewFirewallActioner - створює новий FirewallActioner
func NewFirewallActioner(cfg ActionerConfig) (*FirewallActioner, error) {
	fa := &FirewallActioner{
		projectID: cfg.Params["project_id"].(string),
		timeout:   time.Duration(cfg.Params["timeout"].(int)) * time.Minute,
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
		if err := fa.blockIP(event.IP, priority, description); err != nil {
			return fmt.Errorf("failed to block IP %s: %v", event.IP, err)
		}
		time.AfterFunc(fa.timeout, func() {
			if err := fa.unblockIP(event.IP); err != nil {
				log.Printf("Failed to unblock IP %s: %v", event.IP, err)
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
	// Замінюємо крапки в IP на дефіси та додаємо унікальний суфікс
	safeIP := strings.ReplaceAll(ip, ".", "-")
	ruleName := fmt.Sprintf("block-%s-%d", safeIP, time.Now().UnixNano())
	// Обрізаємо до 63 символів, якщо ім’я занадто довге
	if len(ruleName) > 63 {
		ruleName = ruleName[:63]
		// Видаляємо дефіс із кінця, якщо він є
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
		if strings.Contains(*firewall.Name, safeIP) { // Перевіряємо, чи ім’я включає safeIP
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
