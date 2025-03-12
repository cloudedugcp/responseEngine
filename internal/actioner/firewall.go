package actioner

import (
	"context"
	"fmt"
	"log"
	"time"

	compute "cloud.google.com/go/compute/apiv1"
	"cloud.google.com/go/compute/apiv1/computepb"
	"github.com/cloudedugcp/responseEngine/internal/db"
	"google.golang.org/api/option"
)

// FirewallActioner реалізує блокування IP через GCP Firewall
type FirewallActioner struct {
	client      *compute.FirewallsClient
	projectID   string
	timeout     time.Duration
	db          *db.Database
	description string
}

// NewFirewallActioner створює новий FirewallActioner
func NewFirewallActioner(projectID, credentialsFile string, timeout time.Duration, db *db.Database) (*FirewallActioner, error) {
	ctx := context.Background()
	log.Printf("Initializing FirewallActioner with projectID=%s, credentialsFile=%s, timeout=%s", projectID, credentialsFile, timeout)

	client, err := compute.NewFirewallsRESTClient(ctx, option.WithCredentialsFile(credentialsFile))
	if err != nil {
		return nil, fmt.Errorf("failed to create firewall client: %v", err)
	}
	log.Printf("Firewall client initialized successfully for project %s", projectID)

	return &FirewallActioner{
		client:      client,
		projectID:   projectID,
		timeout:     timeout,
		db:          db,
		description: "Blocked by Response Engine",
	}, nil
}

// Execute блокує IP через фаєрвол
func (fa *FirewallActioner) Execute(event Event, params map[string]interface{}) error {
	var priority int32
	switch p := params["priority"].(type) {
	case float64:
		priority = int32(p)
	case int:
		priority = int32(p)
	case nil:
		return fmt.Errorf("priority is missing in params")
	default:
		return fmt.Errorf("priority must be a number, got %T with value %v", p, p)
	}

	description, _ := params["description"].(string)
	if description != "" {
		fa.description = description
	}

	blockCount, err := fa.db.GetBlockCount(event.IP)
	if err != nil {
		log.Printf("Failed to get block count for IP %s: %v", event.IP, err)
	}
	ruleName := fmt.Sprintf("block-ip-%s-%d", event.IP, blockCount+1)

	firewall := &computepb.Firewall{
		Name:        &ruleName,
		Description: &fa.description,
		Priority:    protoInt32(priority),
		Direction:   protoString("INGRESS"),
		Denied: []*computepb.Denied{
			{
				IPProtocol: protoString("all"),
			},
		},
		SourceRanges: []string{event.IP + "/32"},
	}

	ctx, cancel := context.WithTimeout(context.Background(), fa.timeout)
	defer cancel()

	log.Printf("Inserting firewall rule: name=%s, project=%s, priority=%d", ruleName, fa.projectID, priority)
	op, err := fa.client.Insert(ctx, &computepb.InsertFirewallRequest{
		Project:          fa.projectID,
		FirewallResource: firewall,
	})
	if err != nil {
		log.Printf("Failed to insert firewall rule: %v", err)
		return fmt.Errorf("failed to insert firewall rule: %v", err)
	}

	log.Printf("Waiting for firewall operation to complete")
	if err := op.Wait(ctx); err != nil {
		log.Printf("Failed to wait for firewall operation: %v", err)
		return fmt.Errorf("failed to wait for firewall operation: %v", err)
	}

	log.Printf("Firewall rule %s inserted successfully", ruleName)
	return nil
}

// Name повертає ім’я діяча
func (fa *FirewallActioner) Name() string {
	return "firewall"
}

func protoString(s string) *string {
	return &s
}

func protoInt32(i int32) *int32 {
	return &i
}
