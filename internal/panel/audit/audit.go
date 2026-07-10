package audit

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/lucasile/deft/internal/panel/auth"
	"github.com/lucasile/deft/internal/panel/db"
)

type Event struct {
	User       *auth.User
	Action     string
	NodeID     string
	TargetID   string
	CommandID  string
	Success    bool
	Message    string
	RemoteAddr string
}

type Logger struct {
	db *db.DB
}

func NewLogger(database *db.DB) *Logger {
	return &Logger{db: database}
}

func (l *Logger) Record(event Event) error {
	eventID, err := randomID()
	if err != nil {
		return err
	}

	var userID, username string
	if event.User != nil {
		userID = event.User.ID
		username = event.User.Username
	}

	success := 0
	if event.Success {
		success = 1
	}

	_, err = l.db.Exec(
		`INSERT INTO audit_logs
		 (id, user_id, username, action, node_id, target_id, command_id, success, message, remote_addr, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		eventID,
		userID,
		username,
		event.Action,
		event.NodeID,
		event.TargetID,
		event.CommandID,
		success,
		event.Message,
		event.RemoteAddr,
		time.Now().Unix(),
	)
	if err != nil {
		return fmt.Errorf("failed to write audit log: %w", err)
	}

	return nil
}

func randomID() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("failed to create audit id: %w", err)
	}
	return hex.EncodeToString(b[:]), nil
}
