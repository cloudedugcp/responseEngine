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
	scenario := m.cfg.Scenarios[scenarioName]
	record, err := m.db.GetOrCreateBlockRecord(event.IP)
	if err != nil {
		log.Printf("DB error: %v", err)
		return
	}

	record.TriggerCount++
	if record.TriggerCount >= scenario.TriggerCount {
		m.executeScenario(scenarioName, event.IP, record)
	}
	m.db.UpdateBlockRecord(record)
}

func (m *Manager) executeScenario(scenarioName, ip string, record *models.BlockRecord) {
	scenario := m.cfg.Scenarios[scenarioName]
	buttons := []notifier.SlackButton{}
	for _, actName := range scenario.Actioners {
		buttons = append(buttons, notifier.SlackButton{Name: actName, Value: actName})
	}
	buttons = append(buttons, notifier.SlackButton{Name: "Execute All", Value: "all"})

	m.notifier.SendMessageWithButtons(fmt.Sprintf("IP %s triggered scenario", ip), buttons)

	time.AfterFunc(time.Duration(scenario.WaitTimeout)*time.Second, func() {
		if !m.db.WasActionTaken(ip) {
			m.ExecuteAction("all", ip)
		}
	})
}

func (m *Manager) ExecuteAction(action, ip string) {
	scenario := m.cfg.Scenarios["block_ip"]
	record, _ := m.db.GetOrCreateBlockRecord(ip)

	if action == "all" {
		for _, actName := range scenario.Actioners {
			m.actioners[actName].Execute(ip)
		}
	} else {
		m.actioners[action].Execute(ip)
	}

	record.BlockedAt = time.Now().Unix()
	record.UnblockAfter = time.Now().Unix() + int64(scenario.UnblockAfter*record.BlockCount)
	record.BlockCount++
	m.db.UpdateBlockRecord(record)

	go m.scheduleUnblock(ip, record)
}

func (m *Manager) scheduleUnblock(ip string, record *models.BlockRecord) {
	time.Sleep(time.Until(time.Unix(record.UnblockAfter, 0)))
	m.actioners["gcp_firewall"].Execute(ip) // Логіка розблокування
	record.BlockedAt = 0
	record.TriggerCount = 0
	m.db.UpdateBlockRecord(record)
}
