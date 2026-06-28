// Package store persists every message the hub sends or receives, plus its
// acknowledgement state — the audit/replay backbone OpenEMR core has no
// equivalent for. M1 uses SQLite (pure-Go driver, no cgo); the Store interface
// lets a Postgres impl drop in at M4 without touching callers.
package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

// Direction of a stored message relative to the hub.
type Direction string

const (
	Outbound Direction = "outbound" // hub -> external
	Inbound  Direction = "inbound"  // external -> hub
)

// Message is one archived HL7 message and its ack state.
type Message struct {
	ID        int64
	Channel   string
	Direction Direction
	Type      string // e.g. "ADT^A04"
	ControlID string
	Raw       string
	AckCode   string // AA/AE/AR
	AckText   string
	Status    string // "sent","acked","nacked","received","error"
	CreatedAt time.Time
}

// Store is the message archive.
type Store interface {
	Save(ctx context.Context, m Message) (int64, error)
	SetAck(ctx context.Context, id int64, code, text, status string) error
	List(ctx context.Context, limit int) ([]Message, error)
	Close() error
}

// SQLite is the SQLite-backed Store.
type SQLite struct{ db *sql.DB }

// OpenSQLite opens (and migrates) the SQLite database at path.
func OpenSQLite(path string) (*SQLite, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("store open: %w", err)
	}
	if _, err := db.Exec(schema); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("store migrate: %w", err)
	}
	return &SQLite{db: db}, nil
}

const schema = `
CREATE TABLE IF NOT EXISTS messages (
	id         INTEGER PRIMARY KEY AUTOINCREMENT,
	channel    TEXT NOT NULL DEFAULT '',
	direction  TEXT NOT NULL,
	type       TEXT NOT NULL DEFAULT '',
	control_id TEXT NOT NULL DEFAULT '',
	raw        TEXT NOT NULL,
	ack_code   TEXT NOT NULL DEFAULT '',
	ack_text   TEXT NOT NULL DEFAULT '',
	status     TEXT NOT NULL DEFAULT '',
	created_at TIMESTAMP NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_messages_created ON messages(created_at);
CREATE INDEX IF NOT EXISTS idx_messages_control ON messages(control_id);
`

func (s *SQLite) Save(ctx context.Context, m Message) (int64, error) {
	if m.CreatedAt.IsZero() {
		m.CreatedAt = time.Now().UTC()
	}
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO messages(channel,direction,type,control_id,raw,ack_code,ack_text,status,created_at)
		 VALUES(?,?,?,?,?,?,?,?,?)`,
		m.Channel, m.Direction, m.Type, m.ControlID, m.Raw, m.AckCode, m.AckText, m.Status, m.CreatedAt)
	if err != nil {
		return 0, fmt.Errorf("store save: %w", err)
	}
	return res.LastInsertId()
}

func (s *SQLite) SetAck(ctx context.Context, id int64, code, text, status string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE messages SET ack_code=?, ack_text=?, status=? WHERE id=?`,
		code, text, status, id)
	if err != nil {
		return fmt.Errorf("store set ack: %w", err)
	}
	return nil
}

func (s *SQLite) List(ctx context.Context, limit int) ([]Message, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT id,channel,direction,type,control_id,raw,ack_code,ack_text,status,created_at
		 FROM messages ORDER BY id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("store list: %w", err)
	}
	defer rows.Close()

	var out []Message
	for rows.Next() {
		var m Message
		if err := rows.Scan(&m.ID, &m.Channel, &m.Direction, &m.Type, &m.ControlID,
			&m.Raw, &m.AckCode, &m.AckText, &m.Status, &m.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (s *SQLite) Close() error { return s.db.Close() }
