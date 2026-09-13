package sqlite

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func (r *TaskRepository) Backup(destination string) error {
	if destination == "" {
		return errors.New("backup destination is required")
	}
	if _, err := os.Stat(destination); err == nil {
		return errors.New("backup destination already exists")
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(destination), 0750); err != nil {
		return err
	}
	escaped := strings.ReplaceAll(destination, "'", "''")
	_, err := r.db.Exec("VACUUM INTO '" + escaped + "'")
	return err
}

func (r *TaskRepository) VerifyIntegrity() error {
	var result string
	if err := r.db.QueryRow("PRAGMA integrity_check").Scan(&result); err != nil {
		return err
	}
	if result != "ok" {
		return fmt.Errorf("sqlite integrity check failed: %s", result)
	}
	return nil
}
