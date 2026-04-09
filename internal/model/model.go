package model

import "time"

type Visibility string

const (
	VisibilityPublic  Visibility = "public"
	VisibilityPrivate Visibility = "private"
)

type OrgRole string

const (
	OrgRoleAdmin  OrgRole = "admin"
	OrgRoleMember OrgRole = "member"
)

type OwnerType string

const (
	OwnerTypeUser OwnerType = "user"
	OwnerTypeOrg  OwnerType = "org"
)

type User struct {
	ID           string
	Username     string
	PasswordHash string
	CreatedAt    time.Time
}

type Org struct {
	ID        string
	Name      string
	CreatedAt time.Time
}

type OrgMember struct {
	OrgID  string
	UserID string
	Role   OrgRole
}

type Module struct {
	ID         string
	OwnerName  string
	OwnerType  OwnerType
	Name       string
	Visibility Visibility
	CreatedAt  time.Time
}

type Commit struct {
	ID              string
	ModuleID        string
	StorageKey      string
	Manifest        string
	CreatedByUserID string
	CreatedAt       time.Time
}

type Label struct {
	ID        string
	ModuleID  string
	Name      string
	CommitID  string
	CreatedAt time.Time
	UpdatedAt time.Time
}

type Token struct {
	ID        string
	UserID    string
	Hash      string
	Note      string
	ExpiresAt *time.Time
	CreatedAt time.Time
}
