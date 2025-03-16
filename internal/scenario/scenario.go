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
	cfg           *config.Config
	actioners     map[string]actioner.Actioner
	db            *db.SQLiteDB
	notifier      *notifier.SlackNotifier
	cancel        map[string]chan struct{} // Для скасування notifier timeout
	unblockCancel map[string]chan struct{} // Для скасування unblock таймерів
}

func NewManager(cfg *config.Config, actioners map[string]actioner.Actioner, db *db.SQLiteDB, notifier *notifier.SlackNotifier) *Manager {
	return &Manager{
		cfg:           cfg,
		actioners:     actioners,
		db:            db,
		notifier:      notifier,
		cancel:        make(map[string]chan struct{}),
		unblockCancel: make(map[string]chan struct{}),
	}
}

func (m *Manager) HandleEvent(scenarioName string, event models.Event) {
	log.Printf("Handling event for IP %s, scenario %s", event.IP, scenarioName)
	scenario, exists := m.cfg.Scenarios[scenarioName]
	if !exists {
		log.Printf("Scenario %s not found in config", scenarioName)
		return
	}

	if event.Rule != scenario.Rule {
		log.Printf("Event rule %s does not match scenario rule %s for IP %s", event.Rule, scenario.Rule, event.IP)
		return
	}

	record, err := m.db.GetOrCreateBlockRecord(event.IP)
	if err != nil {
		log.Printf("DB error for IP %s: %v", event.IP, err)
		return
	}

	currentTime := time.Now().Unix()
	if record.BlockedAt == 0 && record.TriggerCount > 0 && (currentTime-record.LastEventTime) > int64(scenario.Params.TriggerWindow*60) {
		log.Printf("Resetting TriggerCount for IP %s due to expired window", event.IP)
		record.TriggerCount = 0
		record.ActionTaken = false // Скидаємо, якщо вікно минув
	}

	// Перевіряємо, чи сценарій уже активний або повідомлення вже відправлено
	if record.ActionTaken {
		log.Printf("Scenario already triggered for IP %s, skipping execution", event.IP)
		return
	}

	record.TriggerCount++
	record.LastEventTime = currentTime
	log.Printf("IP %s: TriggerCount = %d, required = %d", event.IP, record.TriggerCount, scenario.Params.TriggerCount)

	if scenario.Params.TriggerCount <= 0 {
		log.Printf("Invalid trigger_count %d for scenario %s, defaulting to 1", scenario.Params.TriggerCount, scenarioName)
		scenario.Params.TriggerCount = 1
	}

	// Викликаємо сценарій лише коли TriggerCount вперше досягає межі
	if record.TriggerCount == scenario.Params.TriggerCount {
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

	if scenario.Action.Notifier.Enabled && scenario.Action.Notifier.Name == "slack" {
		buttons := []notifier.SlackButton{}
		for _, actName := range scenario.Action.Actioners {
			buttons = append(buttons, notifier.SlackButton{Name: actName, Value: actName})
		}
		buttons = append(buttons, notifier.SlackButton{Name: "Execute All", Value: "all"})

		message := fmt.Sprintf("IP %s triggered scenario %s", ip, scenarioName)
		if err := m.notifier.SendMessageWithButtons(message, buttons); err != nil {
			log.Printf("Failed to send Slack message for IP %s: %v", ip, err)
		} else {
			log.Printf("Slack message sent successfully for IP %s", ip)
		}

		cancelChan := make(chan struct{})
		m.cancel[ip] = cancelChan

		log.Printf("Setting notifier timeout to %d minutes for IP %s", scenario.Action.Notifier.Timeout, ip)
		time.AfterFunc(time.Duration(scenario.Action.Notifier.Timeout)*time.Minute, func() {
			select {
			case <-cancelChan:
				log.Printf("Notifier timeout cancelled for IP %s", ip)
				return
			default:
				updatedRecord, _ := m.db.GetOrCreateBlockRecord(ip)
				if !updatedRecord.ActionTaken {
					log.Printf("No action taken within notifier timeout for IP %s, executing all actioners", ip)
					m.ExecuteAction("all", ip)
					// Оновлюємо повідомлення в Slack після автоматичного блокування
					if err := m.notifier.UpdateMessage(message, "Автоматично виконано всі дії для IP "+ip); err != nil {
						log.Printf("Failed to update Slack message for IP %s: %v", ip, err)
					} else {
						log.Printf("Slack message updated for IP %s after auto-execution", ip)
					}
				} else {
					log.Printf("Action already taken for IP %s within notifier timeout", ip)
				}
			}
			delete(m.cancel, ip)
		})
	} else {
		log.Printf("Notifier disabled for scenario %s, executing all actioners for IP %s", scenarioName, ip)
		m.ExecuteAction("all", ip)
	}
}

func (m *Manager) ExecuteAction(action, ip string) {
	scenario := m.cfg.Scenarios["block_ip"]
	record, _ := m.db.GetOrCreateBlockRecord(ip)

	if record.BlockedAt > 0 && action != "all" {
		log.Printf("IP %s already blocked, skipping action %s", ip, action)
		return
	}

	if action == "all" {
		for _, actName := range scenario.Action.Actioners {
			log.Printf("Executing actioner %s for IP %s", actName, ip)
			if err := m.actioners[actName].Execute(ip); err != nil {
				log.Printf("Failed to execute actioner %s for IP %s: %v", actName, ip, err)
			}
		}
	} else if actioner, ok := m.actioners[action]; ok {
		log.Printf("Executing actioner %s for IP %s", action, ip)
		if err := actioner.Execute(ip); err != nil {
			log.Printf("Failed to execute actioner %s for IP %s: %v", action, ip, err)
			return
		}
	} else {
		log.Printf("Unknown action %s for IP %s", action, ip)
		return
	}

	record.ActionTaken = true
	if action == "gcp_firewall" || action == "all" {
		baseUnblockAfter := int64(scenario.Params.UnblockAfter * 60) // Базовий час у секундах
		multiplier := int64(record.BlockCount + 1)                   // Збільшуємо на основі кількості попередніх блокувань
		record.BlockedAt = time.Now().Unix()
		record.UnblockAfter = time.Now().Unix() + baseUnblockAfter*multiplier
		record.BlockCount++
		if cancelChan, ok := m.cancel[ip]; ok {
			close(cancelChan)
			delete(m.cancel, ip)
			log.Printf("Cancelled notifier timeout for IP %s due to action execution", ip)
		}
		unblockCancelChan := make(chan struct{})
		m.unblockCancel[ip] = unblockCancelChan
		log.Printf("IP %s blocked, unblock after %d seconds (BlockCount: %d)", ip, baseUnblockAfter*multiplier, record.BlockCount)
		go m.scheduleUnblock(ip, record, unblockCancelChan)
	}
	if err := m.db.UpdateBlockRecord(record); err != nil {
		log.Printf("Failed to update block record for IP %s: %v", ip, err)
	}
}

func (m *Manager) scheduleUnblock(ip string, record *models.BlockRecord, cancelChan chan struct{}) {
	select {
	case <-time.After(time.Until(time.Unix(record.UnblockAfter, 0))):
		log.Printf("Unblocking IP %s", ip)
		if firewall, ok := m.actioners["gcp_firewall"].(*actioner.GCPFirewall); ok {
			if err := firewall.Unblock(ip); err != nil {
				log.Printf("Failed to unblock IP %s: %v", ip, err)
			}
		}
		record.BlockedAt = 0
		record.TriggerCount = 0
		record.ActionTaken = false
		if err := m.db.UpdateBlockRecord(record); err != nil {
			log.Printf("Failed to update block record for IP %s after unblock: %v", ip, err)
		}
	case <-cancelChan:
		log.Printf("Unblock timer cancelled for IP %s", ip)
	}
	delete(m.unblockCancel, ip)
}

func (m *Manager) ManualUnblock(ip string) error {
	record, err := m.db.GetOrCreateBlockRecord(ip)
	if err != nil {
		return fmt.Errorf("failed to get block record for IP %s: %v", ip, err)
	}

	if record.BlockedAt == 0 {
		log.Printf("IP %s is not blocked, no action needed", ip)
		return nil
	}

	// Скасовуємо таймер notifier, якщо він є
	if cancelChan, ok := m.cancel[ip]; ok {
		close(cancelChan)
		delete(m.cancel, ip)
		log.Printf("Cancelled notifier timeout for IP %s due to manual unblock", ip)
	}

	// Скасовуємо таймер розблокування, якщо він є
	if unblockCancelChan, ok := m.unblockCancel[ip]; ok {
		close(unblockCancelChan)
		delete(m.unblockCancel, ip)
		log.Printf("Cancelled unblock timer for IP %s due to manual unblock", ip)
	}

	// Виконуємо розблокування
	if firewall, ok := m.actioners["gcp_firewall"].(*actioner.GCPFirewall); ok {
		if err := firewall.Unblock(ip); err != nil {
			return fmt.Errorf("failed to unblock IP %s: %v", ip, err)
		}
	}

	record.BlockedAt = 0
	record.TriggerCount = 0
	record.ActionTaken = false
	if err := m.db.UpdateBlockRecord(record); err != nil {
		return fmt.Errorf("failed to update block record for IP %s: %v", ip, err)
	}

	log.Printf("Successfully manually unblocked IP %s", ip)
	return nil
}
