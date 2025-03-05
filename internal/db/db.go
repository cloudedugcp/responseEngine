package db

import (
	"database/sql"
	"log"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// Database - структура для роботи з локальною БД
type Database struct {
	conn *sql.DB
}

// ActionLog - структура для групування дій за IP
type ActionLog struct {
	IP              string
	LastEvent       string
	AttemptCount    int
	LastAttemptTime time.Time
	BlockTime       time.Time
	UnblockTime     time.Time
	Status          string
	BlockCount      int // Додаємо кількість заблокувань
}

// NewDatabase - створює нову базу даних
func NewDatabase(path string) (*Database, error) {
	conn, err := sql.Open("sqlite3", path)
	if err != nil {
		return nil, err
	}

	_, err = conn.Exec(`
        CREATE TABLE IF NOT EXISTS ip_actions (
            ip TEXT PRIMARY KEY,
            last_event TEXT NOT NULL,
            attempt_count INTEGER NOT NULL DEFAULT 0,
            last_attempt_time DATETIME NOT NULL,
            block_time DATETIME,
            unblock_time DATETIME,
            status TEXT NOT NULL DEFAULT 'active',
            block_count INTEGER NOT NULL DEFAULT 0
        )
    `)
	if err != nil {
		return nil, err
	}

	return &Database{conn: conn}, nil
}

// LogAction - записує або оновлює дію для IP
func (d *Database) LogAction(ip, event, status string, timestamp time.Time) error {
	var exists bool
	err := d.conn.QueryRow("SELECT EXISTS(SELECT 1 FROM ip_actions WHERE ip = ?)", ip).Scan(&exists)
	if err != nil {
		log.Printf("Error checking IP existence: %v", err)
		return err
	}

	if !exists {
		_, err = d.conn.Exec(`
            INSERT INTO ip_actions (ip, last_event, attempt_count, last_attempt_time, status)
            VALUES (?, ?, 1, ?, ?)
        `, ip, event, timestamp, status)
	} else {
		switch status {
		case "received":
			_, err = d.conn.Exec(`
                UPDATE ip_actions
                SET last_event = ?, 
                    attempt_count = attempt_count + 1, 
                    last_attempt_time = ?
                WHERE ip = ?
            `, event, timestamp, ip)
		case "blocked":
			_, err = d.conn.Exec(`
                UPDATE ip_actions
                SET block_time = ?, 
                    status = ?,
                    block_count = block_count + 1
                WHERE ip = ?
            `, timestamp, status, ip)
		case "unblocked":
			_, err = d.conn.Exec(`
                UPDATE ip_actions
                SET unblock_time = ?, 
                    status = ?, 
                    attempt_count = 0
                WHERE ip = ?
            `, timestamp, status, ip)
		}
	}
	if err != nil {
		log.Printf("Error logging action for IP %s: %v", ip, err)
	}
	return err
}

// CountEvents - повертає кількість спроб для IP за період
func (d *Database) CountEvents(ip string, window time.Duration) (int, error) {
	cutoff := time.Now().Add(-window)
	var count int
	err := d.conn.QueryRow(`
        SELECT attempt_count 
        FROM ip_actions 
        WHERE ip = ? AND last_attempt_time >= ?
    `, ip, cutoff).Scan(&count)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	if err != nil {
		log.Printf("Error counting events for IP %s: %v", ip, err)
		return 0, err
	}
	return count, nil
}

// GetBlockCount - повертає кількість заблокувань для IP
func (d *Database) GetBlockCount(ip string) (int, error) {
	var count int
	err := d.conn.QueryRow("SELECT block_count FROM ip_actions WHERE ip = ?", ip).Scan(&count)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	if err != nil {
		log.Printf("Error getting block count for IP %s: %v", ip, err)
		return 0, err
	}
	return count, nil
}

// GetActions - повертає список дій для веб-інтерфейсу
func (d *Database) GetActions() ([]ActionLog, error) {
	rows, err := d.conn.Query(`
        SELECT ip, last_event, attempt_count, last_attempt_time, 
               block_time, unblock_time, status, block_count 
        FROM ip_actions 
        ORDER BY last_attempt_time DESC
    `)
	if err != nil {
		log.Printf("Error querying actions: %v", err)
		return nil, err
	}
	defer rows.Close()

	var actions []ActionLog
	for rows.Next() {
		var a ActionLog
		var blockTime, unblockTime sql.NullTime
		if err := rows.Scan(&a.IP, &a.LastEvent, &a.AttemptCount, &a.LastAttemptTime,
			&blockTime, &unblockTime, &a.Status, &a.BlockCount); err != nil {
			return nil, err
		}
		if blockTime.Valid {
			a.BlockTime = blockTime.Time
		}
		if unblockTime.Valid {
			a.UnblockTime = unblockTime.Time
		}
		actions = append(actions, a)
	}
	return actions, nil
}

// Close - закриває з'єднання з БД
func (d *Database) Close() error {
	return d.conn.Close()
}
