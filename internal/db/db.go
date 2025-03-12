package db

import (
	"database/sql"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// Database представляє базу даних SQLite
type Database struct {
	db *sql.DB
}

// NewDatabase створює нову базу даних
func NewDatabase(path string) (*Database, error) {
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		return nil, err
	}

	_, err = db.Exec(`
        CREATE TABLE IF NOT EXISTS events (
            ip TEXT,
            rule_name TEXT,
            log TEXT,
            timestamp DATETIME
        );
        CREATE TABLE IF NOT EXISTS actions (
            ip TEXT,
            action_type TEXT,
            status TEXT,
            timestamp DATETIME
        );
    `)
	if err != nil {
		return nil, err
	}

	return &Database{db: db}, nil
}

// Close закриває базу даних
func (d *Database) Close() error {
	return d.db.Close()
}

// LogEvent логує подію
func (d *Database) LogEvent(ip, ruleName, logText string, timestamp time.Time) error {
	_, err := d.db.Exec("INSERT INTO events (ip, rule_name, log, timestamp) VALUES (?, ?, ?, ?)", ip, ruleName, logText, timestamp)
	return err
}

// CountEvents підраховує кількість подій за IP за певний час
func (d *Database) CountEvents(ip string, window time.Duration) (int, error) {
	cutoff := time.Now().Add(-window)
	var count int
	err := d.db.QueryRow("SELECT COUNT(*) FROM events WHERE ip = ? AND timestamp >= ?", ip, cutoff).Scan(&count)
	return count, err
}

// LogAction логує дію (block/unblock)
func (d *Database) LogAction(ip, actionType, status string, timestamp time.Time) error {
	_, err := d.db.Exec("INSERT INTO actions (ip, action_type, status, timestamp) VALUES (?, ?, ?, ?)", ip, actionType, status, timestamp)
	if err != nil {
		return err
	}
	if actionType == "block" {
		_, err = d.db.Exec("UPDATE actions SET status = 'blocked' WHERE ip = ? AND action_type = 'block' AND status != 'unblocked'", ip)
	}
	return err
}

// GetBlockCount підраховує кількість блокувань для IP
func (d *Database) GetBlockCount(ip string) (int, error) {
	var count int
	err := d.db.QueryRow("SELECT COUNT(*) FROM actions WHERE ip = ? AND action_type = 'block'", ip).Scan(&count)
	return count, err
}

// ResetAttemptCount скидає кількість спроб для IP
func (d *Database) ResetAttemptCount(ip string) error {
	_, err := d.db.Exec("DELETE FROM events WHERE ip = ?", ip)
	return err
}

// GetIPStats повертає статистику по IP для дашборда
func (d *Database) GetIPStats() ([]IPStats, error) {
	rows, err := d.db.Query(`
        SELECT 
            e.ip, 
            MAX(e.rule_name) as last_event, 
            COUNT(e.ip) as attempt_count, 
            MAX(e.timestamp) as last_attempt_time,
            (SELECT MAX(timestamp) FROM actions WHERE ip = e.ip AND action_type = 'block' AND status = 'blocked') as block_time,
            (SELECT MAX(timestamp) FROM actions WHERE ip = e.ip AND action_type = 'unblock' AND status = 'unblocked') as unblock_time,
            (SELECT COUNT(*) FROM actions WHERE ip = e.ip AND action_type = 'block') as block_count,
            COALESCE((SELECT status FROM actions WHERE ip = e.ip ORDER BY timestamp DESC LIMIT 1), 'pending') as status
        FROM events e
        GROUP BY e.ip
    `)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var stats []IPStats
	for rows.Next() {
		var s IPStats
		var blockTime, unblockTime sql.NullTime
		if err := rows.Scan(&s.IP, &s.LastEvent, &s.AttemptCount, &s.LastAttemptTime, &blockTime, &unblockTime, &s.BlockCount, &s.Status); err != nil {
			return nil, err
		}
		if blockTime.Valid {
			s.BlockTime = blockTime.Time
		}
		if unblockTime.Valid {
			s.UnblockTime = unblockTime.Time
		}
		stats = append(stats, s)
	}
	return stats, nil
}

// IPStats представляє статистику по IP
type IPStats struct {
	IP              string
	LastEvent       string
	AttemptCount    int
	LastAttemptTime time.Time
	BlockTime       time.Time
	UnblockTime     time.Time
	BlockCount      int
	Status          string
}
