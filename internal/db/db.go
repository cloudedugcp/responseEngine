package db

import (
	"database/sql"
	"fmt"
	"time"

	_ "github.com/mattn/go-sqlite3" // Драйвер SQLite
)

// Database представляє базу даних SQLite
type Database struct {
	db *sql.DB
}

// IPLog представляє запис у таблиці ip_logs
type IPLog struct {
	IP              string
	LastEvent       string
	AttemptCount    int
	LastAttemptTime time.Time
	BlockTime       *time.Time // NULLable
	UnblockTime     *time.Time // NULLable
	BlockCount      int
	Status          string
}

// NewDatabase створює нову базу даних SQLite
func NewDatabase(path string) (*Database, error) {
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		return nil, fmt.Errorf("failed to open SQLite database: %v", err)
	}

	err = createTables(db)
	if err != nil {
		return nil, fmt.Errorf("failed to create tables: %v", err)
	}

	return &Database{db: db}, nil
}

// createTables створює таблицю ip_logs
func createTables(db *sql.DB) error {
	_, err := db.Exec(`
        CREATE TABLE IF NOT EXISTS ip_logs (
            ip TEXT PRIMARY KEY,
            last_event TEXT NOT NULL,
            attempt_count INTEGER NOT NULL DEFAULT 0,
            last_attempt_time DATETIME NOT NULL,
            block_time DATETIME,
            unblock_time DATETIME,
            block_count INTEGER NOT NULL DEFAULT 0,
            status TEXT NOT NULL DEFAULT 'pending'
        )
    `)
	if err != nil {
		return fmt.Errorf("failed to create ip_logs table: %v", err)
	}
	return nil
}

// LogEvent оновлює або додає запис про подію для IP
func (d *Database) LogEvent(ip, rule, log string, timestamp time.Time) error {
	query := `
        INSERT INTO ip_logs (ip, last_event, attempt_count, last_attempt_time, status)
        VALUES (?, ?, 1, ?, 'pending')
        ON CONFLICT(ip) DO UPDATE SET
            last_event = ?,
            attempt_count = attempt_count + 1,
            last_attempt_time = ?,
            status = CASE WHEN block_time IS NOT NULL THEN 'blocked' ELSE 'pending' END
    `
	_, err := d.db.Exec(query, ip, rule, ip, rule, timestamp, timestamp)
	if err != nil {
		return fmt.Errorf("failed to log event for IP %s: %v", ip, err)
	}
	return nil
}

// CountEvents повертає кількість подій за IP за певний часовий інтервал
func (d *Database) CountEvents(ip string, window time.Duration) (int, error) {
	query := `
        SELECT attempt_count
        FROM ip_logs
        WHERE ip = ? AND last_attempt_time >= ?
    `
	cutoff := time.Now().UTC().Add(-window)
	var count int
	err := d.db.QueryRow(query, ip, cutoff).Scan(&count)
	if err == sql.ErrNoRows {
		return 0, nil // Якщо запису немає, повертаємо 0
	}
	if err != nil {
		return 0, fmt.Errorf("failed to count events for IP %s: %v", ip, err)
	}
	return count, nil
}

// LogAction оновлює статус і час блокування/розблокування для IP
func (d *Database) LogAction(ip, actionType, status string, timestamp time.Time) error {
	var query string
	if actionType == "block" && status == "blocked" {
		query = `
            UPDATE ip_logs
            SET block_time = ?,
                block_count = block_count + 1,
                status = 'blocked'
            WHERE ip = ?
        `
	} else if actionType == "store" && status == "stored" {
		query = `
            UPDATE ip_logs
            SET status = 'pending'
            WHERE ip = ?
        `
	} else {
		return fmt.Errorf("unsupported action type/status: %s/%s", actionType, status)
	}

	result, err := d.db.Exec(query, timestamp, ip)
	if err != nil {
		return fmt.Errorf("failed to log action for IP %s: %v", ip, err)
	}
	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("no record found for IP %s to update action", ip)
	}
	return nil
}

// GetActions повертає список усіх записів із ip_logs
func (d *Database) GetActions() ([]IPLog, error) {
	query := `
        SELECT ip, last_event, attempt_count, last_attempt_time, block_time, unblock_time, block_count, status
        FROM ip_logs
        ORDER BY last_attempt_time DESC
    `
	rows, err := d.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query ip_logs: %v", err)
	}
	defer rows.Close()

	var logs []IPLog
	for rows.Next() {
		var l IPLog
		var blockTime, unblockTime sql.NullTime
		if err := rows.Scan(&l.IP, &l.LastEvent, &l.AttemptCount, &l.LastAttemptTime, &blockTime, &unblockTime, &l.BlockCount, &l.Status); err != nil {
			return nil, fmt.Errorf("failed to scan ip_log: %v", err)
		}
		if blockTime.Valid {
			l.BlockTime = &blockTime.Time
		}
		if unblockTime.Valid {
			l.UnblockTime = &unblockTime.Time
		}
		logs = append(logs, l)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating ip_logs: %v", err)
	}
	return logs, nil
}

// GetBlockCount повертає кількість блокувань для заданого IP
func (d *Database) GetBlockCount(ip string) (int, error) {
	query := `
        SELECT block_count
        FROM ip_logs
        WHERE ip = ?
    `
	var count int
	err := d.db.QueryRow(query, ip).Scan(&count)
	if err == sql.ErrNoRows {
		return 0, nil // Якщо запису немає, повертаємо 0
	}
	if err != nil {
		return 0, fmt.Errorf("failed to get block count for IP %s: %v", ip, err)
	}
	return count, nil
}

// Close закриває з’єднання з базою даних
func (d *Database) Close() error {
	if err := d.db.Close(); err != nil {
		return fmt.Errorf("failed to close database: %v", err)
	}
	return nil
}
