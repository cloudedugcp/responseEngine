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
	LastEvent       string    // Остання подія (rule)
	AttemptCount    int       // Кількість спроб
	LastAttemptTime time.Time // Час останньої спроби
	BlockTime       time.Time // Час блокування (може бути порожнім)
	UnblockTime     time.Time // Час розблокування (може бути порожнім)
	Status          string    // Статус: "active", "blocked", "unblocked"
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
            status TEXT NOT NULL DEFAULT 'active'
        )
    `)
	if err != nil {
		return nil, err
	}

	return &Database{conn: conn}, nil
}

// LogAction - записує або оновлює дію для IP
func (d *Database) LogAction(ip, event, status string, timestamp time.Time) error {
	// Перевіряємо, чи існує запис для IP
	var exists bool
	err := d.conn.QueryRow("SELECT EXISTS(SELECT 1 FROM ip_actions WHERE ip = ?)", ip).Scan(&exists)
	if err != nil {
		log.Printf("Error checking IP existence: %v", err)
		return err
	}

	if !exists {
		// Якщо IP новий, вставляємо запис
		_, err = d.conn.Exec(`
            INSERT INTO ip_actions (ip, last_event, attempt_count, last_attempt_time, status)
            VALUES (?, ?, 1, ?, ?)
        `, ip, event, timestamp, status)
	} else {
		// Якщо IP існує, оновлюємо відповідні поля
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
                    status = ?
                WHERE ip = ?
            `, timestamp, status, ip)
		case "unblocked":
			_, err = d.conn.Exec(`
                UPDATE ip_actions
                SET unblock_time = ?, 
                    status = ?, 
                    attempt_count = 0  -- Скидаємо лічильник після розблокування
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
		return 0, nil // Якщо запису немає, повертаємо 0
	}
	if err != nil {
		log.Printf("Error counting events for IP %s: %v", ip, err)
		return 0, err
	}
	return count, nil
}

// GetActions - повертає список дій для веб-інтерфейсу
func (d *Database) GetActions() ([]ActionLog, error) {
	rows, err := d.conn.Query(`
        SELECT ip, last_event, attempt_count, last_attempt_time, 
               block_time, unblock_time, status 
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
		var blockTime, unblockTime sql.NullTime // Для підтримки NULL
		if err := rows.Scan(&a.IP, &a.LastEvent, &a.AttemptCount, &a.LastAttemptTime,
			&blockTime, &unblockTime, &a.Status); err != nil {
			return nil, err
		}
		a.BlockTime = blockTime.Time
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
