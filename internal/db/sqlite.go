package db

import (
	"database/sql"

	"github.com/cloudedugcp/responseEngine/pkg/models"

	_ "github.com/mattn/go-sqlite3"
)

type SQLiteDB struct {
	db *sql.DB
}

func NewSQLiteDB(path string) (*SQLiteDB, error) {
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		return nil, err
	}
	_, err = db.Exec(`
        CREATE TABLE IF NOT EXISTS blocks (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            ip TEXT UNIQUE,
            blocked_at INTEGER,
            unblock_after INTEGER,
            block_count INTEGER,
            trigger_count INTEGER,
            last_event_time INTEGER DEFAULT 0,
			action_taken BOOLEAN DEFAULT 0
        )
    `)
	return &SQLiteDB{db}, err
}

func (d *SQLiteDB) GetOrCreateBlockRecord(ip string) (*models.BlockRecord, error) {
	var record models.BlockRecord
	err := d.db.QueryRow("SELECT id, ip, blocked_at, unblock_after, block_count, trigger_count, last_event_time, action_taken FROM blocks WHERE ip = ?", ip).Scan(
		&record.ID, &record.IP, &record.BlockedAt, &record.UnblockAfter, &record.BlockCount, &record.TriggerCount, &record.LastEventTime, &record.ActionTaken,
	)
	if err != nil && err != sql.ErrNoRows {
		return nil, err
	}
	if err == sql.ErrNoRows {
		res, err := d.db.Exec("INSERT INTO blocks (ip, blocked_at, unblock_after, block_count, trigger_count, last_event_time, action_taken) VALUES (?, 0, 0, 0, 0, 0, 0)", ip)
		if err != nil {
			return nil, err
		}
		id, _ := res.LastInsertId()
		record = models.BlockRecord{ID: int(id), IP: ip}
	}
	return &record, nil
}

func (d *SQLiteDB) UpdateBlockRecord(record *models.BlockRecord) error {
	_, err := d.db.Exec("UPDATE blocks SET blocked_at = ?, unblock_after = ?, block_count = ?, trigger_count = ?, last_event_time = ?, action_taken = ? WHERE ip = ?",
		record.BlockedAt, record.UnblockAfter, record.BlockCount, record.TriggerCount, record.LastEventTime, record.ActionTaken, record.IP)
	return err
}

func (d *SQLiteDB) WasActionTaken(ip string) bool {
	var actionTaken bool
	d.db.QueryRow("SELECT action_taken FROM blocks WHERE ip = ?", ip).Scan(&actionTaken)
	return actionTaken
}

func (d *SQLiteDB) GetAllRecords() ([]models.BlockRecord, error) {
	rows, err := d.db.Query("SELECT id, ip, blocked_at, unblock_after, block_count, trigger_count, last_event_time FROM blocks")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []models.BlockRecord
	for rows.Next() {
		var r models.BlockRecord
		if err := rows.Scan(&r.ID, &r.IP, &r.BlockedAt, &r.UnblockAfter, &r.BlockCount, &r.TriggerCount, &r.LastEventTime); err != nil {
			return nil, err
		}
		records = append(records, r)
	}
	return records, nil
}
