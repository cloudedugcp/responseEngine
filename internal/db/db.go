package db

import (
	"database/sql"
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
            timestamp DATETIME NOT NULL
        )
    `)
	if err != nil {
		return nil, err
	}

	return &Database{conn: conn}, nil
}

// LogAction - записує дію в БД
func (d *Database) LogAction(ip, action string, timestamp time.Time) error {
	_, err := d.conn.Exec("INSERT INTO actions (ip, action, timestamp) VALUES (?, ?, ?)",
		ip, action, timestamp)
	return err
}

// CountEvents - підраховує кількість подій для IP за певний період
func (d *Database) CountEvents(ip string, window time.Duration) (int, error) {
	cutoff := time.Now().Add(-window)
	var count int
	err := d.conn.QueryRow(
		"SELECT COUNT(*) FROM actions WHERE ip = ? AND timestamp >= ?",
		ip, cutoff,
	).Scan(&count)
	return count, err
}

// GetActions - повертає список дій для веб-інтерфейсу
func (d *Database) GetActions() ([]ActionLog, error) {
	rows, err := d.conn.Query("SELECT ip, action, timestamp FROM actions ORDER BY timestamp DESC")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var actions []ActionLog
	for rows.Next() {
		var a ActionLog
		if err := rows.Scan(&a.IP, &a.Action, &a.Timestamp); err != nil {
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
