package db

import (
	"database/sql"
	"log"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// Database представляє базу даних SQLite
type Database struct {
	db *sql.DB
}

// NewDatabase ініціалізує нову базу даних
func NewDatabase(path string) (*Database, error) {
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		return nil, err
	}

	// Ініціалізація таблиць
	_, err = db.Exec(`
        CREATE TABLE IF NOT EXISTS events (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            ip TEXT,
            rule_name TEXT,
            log TEXT,
            timestamp DATETIME
        );
        CREATE TABLE IF NOT EXISTS actions (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            ip TEXT,
            action TEXT,
            status TEXT,
            timestamp DATETIME,
            block_time DATETIME,
            unblock_time DATETIME
        );
    `)
	if err != nil {
		return nil, err
	}

	return &Database{db: db}, nil
}

// DB повертає *sql.DB для зовнішнього використання
func (d *Database) DB() *sql.DB {
	return d.db
}

// Close закриває базу даних
func (d *Database) Close() error {
	return d.db.Close()
}

// LogEvent логує подію
func (d *Database) LogEvent(ip, ruleName, logText string, timestamp time.Time) error {
	formattedTime := timestamp.Format("2006-01-02 15:04:05") // Без часового поясу
	_, err := d.db.Exec("INSERT INTO events (ip, rule_name, log, timestamp) VALUES (?, ?, ?, ?)", ip, ruleName, logText, formattedTime)
	if err != nil {
		log.Printf("Error inserting event: ip=%s, rule=%s, log=%s, timestamp=%s, err=%v", ip, ruleName, logText, formattedTime, err)
	} else {
		log.Printf("Event logged: ip=%s, rule=%s, log=%s, timestamp=%s", ip, ruleName, logText, formattedTime)
	}
	return err
}

// CountEvents підраховує кількість подій
func (d *Database) CountEvents(ip string, window time.Duration) (int, error) {
	cutoff := time.Now().Add(-window)
	var count int
	err := d.db.QueryRow("SELECT COUNT(*) FROM events WHERE ip = ? AND timestamp >= ?", ip, cutoff).Scan(&count)
	if err != nil {
		log.Printf("Error counting events: %v", err)
		return 0, err
	}
	return count, nil
}

// LogAction логує дію
func (d *Database) LogAction(ip, action, status string, timestamp time.Time) error {
	_, err := d.db.Exec("INSERT INTO actions (ip, action, status, timestamp) VALUES (?, ?, ?, ?)", ip, action, status, timestamp)
	return err
}

// GetBlockCount повертає кількість блокувань
func (d *Database) GetBlockCount(ip string) (int, error) {
	var count int
	err := d.db.QueryRow("SELECT COUNT(*) FROM actions WHERE ip = ? AND action = 'block'", ip).Scan(&count)
	if err != nil {
		return 0, err
	}
	return count, nil
}

// ResetAttemptCount скидає лічильник спроб для IP
func (d *Database) ResetAttemptCount(ip string) error {
	_, err := d.db.Exec("DELETE FROM events WHERE ip = ?", ip)
	if err != nil {
		log.Printf("Failed to reset attempt count for IP %s: %v", ip, err)
		return err
	}
	log.Printf("Attempt count reset for IP %s", ip)
	return nil
}
