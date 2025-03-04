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

// ActionLog - структура для запису дій
type ActionLog struct {
	IP        string
	Action    string
	Status    string // Додаємо статус (blocked/unblocked/event)
	Timestamp time.Time
}

// NewDatabase - створює нову базу даних
func NewDatabase(path string) (*Database, error) {
	conn, err := sql.Open("sqlite3", path)
	if err != nil {
		return nil, err
	}

	_, err = conn.Exec(`
        CREATE TABLE IF NOT EXISTS actions (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            ip TEXT NOT NULL,
            action TEXT NOT NULL,
            status TEXT NOT NULL,
            timestamp DATETIME NOT NULL
        )
    `)
	if err != nil {
		return nil, err
	}

	return &Database{conn: conn}, nil
}

// LogAction - записує дію в БД
func (d *Database) LogAction(ip, action, status string, timestamp time.Time) error {
	_, err := d.conn.Exec("INSERT INTO actions (ip, action, status, timestamp) VALUES (?, ?, ?, ?)",
		ip, action, status, timestamp)
	return err
}

// CountEvents - підраховує кількість подій для IP за певний період
func (d *Database) CountEvents(ip string, window time.Duration) (int, error) {
	cutoff := time.Now().Add(-window)
	var count int
	err := d.conn.QueryRow(
		"SELECT COUNT(*) FROM actions WHERE ip = ? AND timestamp >= ? AND action = 'event'",
		ip, cutoff,
	).Scan(&count)
	if err != nil {
		log.Printf("Error counting events for IP %s: %v", ip, err) // Логуємо помилку
		return 0, err
	}
	return count, nil
}

// GetActions - повертає список дій для веб-інтерфейсу
func (d *Database) GetActions() ([]ActionLog, error) {
	rows, err := d.conn.Query("SELECT ip, action, status, timestamp FROM actions ORDER BY timestamp DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var actions []ActionLog
	for rows.Next() {
		var a ActionLog
		if err := rows.Scan(&a.IP, &a.Action, &a.Status, &a.Timestamp); err != nil {
			return nil, err
		}
		actions = append(actions, a)
	}
	return actions, nil
}

// Close - закриває з'єднання з БД
func (d *Database) Close() error {
	return d.conn.Close()
}
