package models

import "time"

// UserRecord represents the structure of a user's record in the local store.
// It's expanded to hold more useful data for reference.
type UserRecord struct {
	SCIMID                string     `json:"scim_id"`
	Email                 string     `json:"email"`
	Status                string     `json:"status"` // e.g., "active" or "inactive"
	Name                  SCIMName   `json:"name"`
	Title                 string     `json:"title,omitempty"`
	Organization          string     `json:"organization,omitempty"`
	DeactivationTimestamp *time.Time `json:"deactivation_timestamp,omitempty"`
}

// GroupRecord represents the structure of a group's record in the local store.
type GroupRecord struct {
	SCIMID string `json:"scim_id"`
}

// AuditEvent represents a single entry in the audit log.
type AuditEvent struct {
	Timestamp time.Time `json:"timestamp"`
	UseCase   string    `json:"use_case"`
	Target    string    `json:"target"`
	Status    string    `json:"status"`
	Details   string    `json:"details,omitempty"`
}

// JobTask represents a single task in a bulk processing queue.
type JobTask struct {
	Type   string      `json:"type"`   // e.g., "update", "deactivate", "add-to-group", "remove-from-group"
	Target string      `json:"target"` // The user's ePPN
	Data   interface{} `json:"data"`   // For "update", a map[string]interface{}. For group ops, the group name.
	Status string      `json:"status"` // "pending", "completed", "failed"
}

// --- SCIM API Models ---

// SCIMUser represents a user object as defined by the SCIM protocol.
type SCIMUser struct {
	ID             string            `json:"id,omitempty"`
	Schemas        []string          `json:"schemas"`
	UserName       string            `json:"userName"`
	Name           SCIMName          `json:"name"`
	Emails         []SCIMEmail       `json:"emails"`
	Active         bool              `json:"active"`
	Title          string            `json:"title,omitempty"`
	EnterpriseData EnterpriseUserExt `json:"urn:ietf:params:scim:schemas:extension:enterprise:2.0:User,omitempty"`
}

type SCIMName struct {
	Formatted  string `json:"formatted,omitempty"`
	FamilyName string `json:"familyName,omitempty"`
	GivenName  string `json:"givenName,omitempty"`
}

type SCIMEmail struct {
	Value   string `json:"value"`
	Type    string `json:"type"`
	Primary bool   `json:"primary"`
}

// EnterpriseUserExt holds the enterprise user extension data.
type EnterpriseUserExt struct {
	Organization string `json:"organization,omitempty"`
}

// SCIMPatchOp represents a single PATCH operation.
type SCIMPatchOp struct {
	Op    string      `json:"op"`              // "replace", "add", "remove"
	Path  string      `json:"path,omitempty"`  // e.g., "title", "active", `members[value eq "some-id"]`
	Value interface{} `json:"value,omitempty"` // e.g., "Engineer", false, or a slice of members
}

// SCIMGroup represents a group object from the SCIM API.
type SCIMGroup struct {
	ID          string `json:"id,omitempty"`
	DisplayName string `json:"displayName"`
}

// ListResponse is a generic structure for SCIM list responses (for users, groups, etc.).
type ListResponse struct {
	TotalResults int           `json:"totalResults"`
	ItemsPerPage int           `json:"itemsPerPage"`
	StartIndex   int           `json:"startIndex"`
	Resources    []interface{} `json:"Resources"`
}
