package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/SmartSuiteFoundry/scim-mediator/pkg/models"
)

const (
	usersFile  = "users.json"
	groupsFile = "groups.json"
	auditFile  = "audit.log"
)

// Store manages the file-based System of Record.
type Store struct {
	dataDir string
	mu      sync.Mutex
}

// NewStore creates a new store manager. It ensures the data directory exists.
func NewStore(dataDir string) (*Store, error) {
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("could not create data directory %s: %w", dataDir, err)
	}
	return &Store{dataDir: dataDir}, nil
}

// LoadUsers reads the users.json file and returns the data.
func (s *Store) LoadUsers() (map[string]models.UserRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	path := filepath.Join(s.dataDir, usersFile)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return make(map[string]models.UserRecord), nil // Return empty map if file doesn't exist
		}
		return nil, fmt.Errorf("failed to read users file: %w", err)
	}

	var users map[string]models.UserRecord
	if err := json.Unmarshal(data, &users); err != nil {
		return nil, fmt.Errorf("failed to unmarshal users data: %w", err)
	}
	return users, nil
}

// SaveUsers writes the provided user map to the users.json file.
func (s *Store) SaveUsers(users map[string]models.UserRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := json.MarshalIndent(users, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal users data: %w", err)
	}

	path := filepath.Join(s.dataDir, usersFile)
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write users file: %w", err)
	}
	return nil
}

// LoadGroups reads the groups.json file and returns the data.
func (s *Store) LoadGroups() (map[string]models.GroupRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	path := filepath.Join(s.dataDir, groupsFile)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return make(map[string]models.GroupRecord), nil // Return empty map if file doesn't exist
		}
		return nil, fmt.Errorf("failed to read groups file: %w", err)
	}

	var groups map[string]models.GroupRecord
	if err := json.Unmarshal(data, &groups); err != nil {
		return nil, fmt.Errorf("failed to unmarshal groups data: %w", err)
	}
	return groups, nil
}

// SaveGroups writes the provided group map to the groups.json file.
func (s *Store) SaveGroups(groups map[string]models.GroupRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := json.MarshalIndent(groups, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal groups data: %w", err)
	}

	path := filepath.Join(s.dataDir, groupsFile)
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write groups file: %w", err)
	}
	return nil
}

// AppendToAuditLog appends a new event to the audit log file.
func (s *Store) AppendToAuditLog(event models.AuditEvent) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("failed to marshal audit event: %w", err)
	}

	path := filepath.Join(s.dataDir, auditFile)
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open audit log for writing: %w", err)
	}
	defer f.Close()

	if _, err := f.WriteString(string(data) + "\n"); err != nil {
		return fmt.Errorf("failed to write to audit log: %w", err)
	}

	return nil
}
