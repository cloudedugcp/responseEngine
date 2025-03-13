package scenario

import (
	"fmt"
	"log"
	"time"

	"github.com/cloudedugcp/responseEngine/internal/actioner"
	"github.com/cloudedugcp/responseEngine/internal/config"
	"github.com/cloudedugcp/responseEngine/internal/db"
	"github.com/cloudedugcp/responseEngine/internal/notifier"
	"github.com/cloudedugcp/responseEngine/pkg/models"
)

type Manager struct {
	cfg       *config.Config
	actioners map[string]actioner.Actioner
	db        *db.SQLiteDB
	notifier  *notifier.SlackNotifier
}

func NewManager(cfg *config.Config, actioners map[string]actioner.Actioner, db *db.SQLiteDB, notifier *notifier.SlackNotifier) *Manager {
	return &Manager{cfg, actioners, db, notifier}
}

func (m *Manager) HandleEvent(scenarioName string, event models.Event) {
	log.Printf("Handling event for IP %s, scenario %s", event.IP, scenarioName)
	scenario, exists := m.cfg.Scenarios[scenarioName]
	if !exists {
		log.Printf("Scenario %s not found in config", scenarioName)
		return
	}

	record, err := m.db.GetOrCreateBlockRecord(event.IP)
	if err != nil {
		log.Printf("DB error for IP %s: %v", event.IP, err)
		return
	}

	// Додаємо логіку для trigger_window
	currentTime := time.Now().Unix()
	if record.BlockedAt == 0 && record.TriggerCount > 0 && (currentTime-record.LastEventTime) > int64(scenario.TriggerWindow) {
		log.Printf("Resetting TriggerCount for IP %s due to expired window", event.IP)
		record.TriggerCount = 0
	}

	record.TriggerCount++
	record.LastEventTime = currentTime // Оновлюємо час останньої події
	log.Printf("IP %s: TriggerCount = %d, required = %d", event.IP, record.TriggerCount, scenario.TriggerCount)

	if record.TriggerCount >= scenario.TriggerCount {
		log.Printf("Trigger threshold reached for IP %s, executing scenario", event.IP)
		m.executeScenario(scenarioName, event.IP, record)
	}
	if err := m.db.UpdateBlockRecord(record); err != nil {
		log.Printf("Failed to update DB record for IP %s: %v", event.IP, err)
	}
}

func (m *Manager) executeScenario(scenarioName, ip string, record *models.BlockRecord) {
	scenario := m.cfg.Scenarios[scenarioName]
	log.Printf("Executing scenario %s for IP %s", scenarioName, ip)

	buttons := []notifier.SlackButton{}
	for _, actName := range scenario.Actioners {
		buttons = append(buttons, notifier.SlackButton{Name: actName, Value: actName})
	}
	buttons = append(buttons, notifier.SlackButton{Name: "Execute All", Value: "all"})

	if err := m.notifier.SendMessageWithButtons(fmt.Sprintf("IP %s triggered scenario %s", ip, scenarioName), buttons); err != nil {
		log.Printf("Failed to send Slack message for IP %s: %v", ip, err)
	} else {
		log.Printf("Slack message sent successfully for IP %s", ip)
	}

	time.AfterFunc(time.Duration(scenario.WaitTimeout)*time.Second, func() {
		if !m.db.WasActionTaken(ip) {
			log.Printf("No action taken within timeout for IP %s, executing all actioners", ip)
			m.ExecuteAction("all", ip)
		} else {
			log.Printf("Action already taken for IP %s within timeout", ip)
		}
	})
}

func (m *Manager) ExecuteAction(action, ip string) {
	scenario := m.cfg.Scenarios["block_ip"]
	record, _ := m.db.GetOrCreateBlockRecord(ip)

	// Перевіряємо, чи IP уже заблоковано
	if record.BlockedAt > 0 && action != "all" {
		log.Printf("IP %s already blocked, skipping action %s", ip, action)
		return
	}

	if action == "all" {
		for _, actName := range scenario.Actioners {
			if err := m.actioners[actName].Execute(ip); err != nil {
				log.Printf("Failed to execute actioner %s for IP %s: %v", actName, ip, err)
			}
		}
	} else if actioner, ok := m.actioners[action]; ok {
		if err := actioner.Execute(ip); err != nil {
			log.Printf("Failed to execute actioner %s for IP %s: %v", action, ip, err)
			return
		}
	} else {
		log.Printf("Unknown action %s for IP %s", action, ip)
		return
	}

	// Оновлюємо запис у базі лише для блокуючих дій
	if action == "gcp_firewall" || action == "all" {
		record.BlockedAt = time.Now().Unix()
		record.UnblockAfter = time.Now().Unix() + int64(scenario.UnblockAfter*record.BlockCount)
		record.BlockCount++
		if err := m.db.UpdateBlockRecord(record); err != nil {
			log.Printf("Failed to update block record for IP %s: %v", ip, err)
		}
		go m.scheduleUnblock(ip, record)
	}
}

func (m *Manager) scheduleUnblock(ip string, record *models.BlockRecord) {
	time.Sleep(time.Until(time.Unix(record.UnblockAfter, 0)))
	log.Printf("Unblocking IP %s", ip)
	m.actioners["gcp_firewall"].Execute(ip) // Логіка розблокування
	record.BlockedAt = 0
	record.TriggerCount = 0
	m.db.UpdateBlockRecord(record)
}
