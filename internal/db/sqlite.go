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
            trigger_count INTEGER
        )
    `)
	return &SQLiteDB{db}, err
}

func (d *SQLiteDB) GetOrCreateBlockRecord(ip string) (*models.BlockRecord, error) {
	record := &models.BlockRecord{IP: ip}
	row := d.db.QueryRow("SELECT id, blocked_at, unblock_after, block_count, trigger_count FROM blocks WHERE ip = ?", ip)
	if err := row.Scan(&record.ID, &record.BlockedAt, &record.UnblockAfter, &record.BlockCount, &record.TriggerCount); err != nil {
		if err == sql.ErrNoRows {
			_, err = d.db.Exec("INSERT INTO blocks (ip, blocked_at, unblock_after, block_count, trigger_count) VALUES (?, 0, 0, 0, 0)", ip)
			return record, err
		}
		return nil, err
	}
	return record, nil
}

func (d *SQLiteDB) UpdateBlockRecord(record *models.BlockRecord) error {
	_, err := d.db.Exec("UPDATE blocks SET blocked_at = ?, unblock_after = ?, block_count = ?, trigger_count = ? WHERE ip = ?",
		record.BlockedAt, record.UnblockAfter, record.BlockCount, record.TriggerCount, record.IP)
	return err
}

func (d *SQLiteDB) WasActionTaken(ip string) bool {
	var blockedAt int64
	d.db.QueryRow("SELECT blocked_at FROM blocks WHERE ip = ?", ip).Scan(&blockedAt)
	return blockedAt > 0
}

func (d *SQLiteDB) GetAllRecords() ([]models.BlockRecord, error) {
	rows, err := d.db.Query("SELECT id, ip, blocked_at, unblock_after, block_count, trigger_count FROM blocks")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []models.BlockRecord
	for rows.Next() {
		var r models.BlockRecord
		if err := rows.Scan(&r.ID, &r.IP, &r.BlockedAt, &r.UnblockAfter, &r.BlockCount, &r.TriggerCount); err != nil {
			return nil, err
		}
		records = append(records, r)
	}
	return records, nil
}
