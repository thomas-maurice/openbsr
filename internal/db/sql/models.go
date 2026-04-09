package sql

import (
	"time"

	"github.com/thomas-maurice/openbsr/internal/model"
)

type User struct {
	ID           string `gorm:"primaryKey"`
	Username     string `gorm:"uniqueIndex;not null"`
	PasswordHash string `gorm:"not null"`
	CreatedAt    time.Time
}

type Org struct {
	ID        string `gorm:"primaryKey"`
	Name      string `gorm:"uniqueIndex;not null"`
	CreatedAt time.Time
}

type OrgMember struct {
	OrgID  string    `gorm:"primaryKey;not null"`
	UserID string    `gorm:"primaryKey;not null"`
	Role   string    `gorm:"not null"`
}

type Module struct {
	ID         string `gorm:"primaryKey"`
	OwnerName  string `gorm:"not null;uniqueIndex:idx_owner_name"`
	OwnerType  string `gorm:"not null"`
	Name       string `gorm:"not null;uniqueIndex:idx_owner_name"`
	Visibility string `gorm:"not null;default:private"`
	CreatedAt  time.Time
}

type Commit struct {
	ID              string `gorm:"primaryKey;not null"`
	ModuleID        string `gorm:"primaryKey;not null;index"`
	StorageKey      string `gorm:"not null"`
	Manifest        string
	CreatedByUserID string
	CreatedAt       time.Time
}

type Label struct {
	ID        string `gorm:"primaryKey"`
	ModuleID  string `gorm:"not null;uniqueIndex:idx_module_label"`
	Name      string `gorm:"not null;uniqueIndex:idx_module_label"`
	CommitID  string `gorm:"not null"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

type Token struct {
	ID        string `gorm:"primaryKey"`
	UserID    string `gorm:"not null;index"`
	Hash      string `gorm:"uniqueIndex;not null"`
	Note      string
	ExpiresAt *time.Time
	CreatedAt time.Time
}

// --- converters ---

func userToModel(u *User) *model.User {
	return &model.User{ID: u.ID, Username: u.Username, PasswordHash: u.PasswordHash, CreatedAt: u.CreatedAt}
}

func userFromModel(u *model.User) *User {
	return &User{ID: u.ID, Username: u.Username, PasswordHash: u.PasswordHash, CreatedAt: u.CreatedAt}
}

func orgToModel(o *Org) *model.Org {
	return &model.Org{ID: o.ID, Name: o.Name, CreatedAt: o.CreatedAt}
}

func orgFromModel(o *model.Org) *Org {
	return &Org{ID: o.ID, Name: o.Name, CreatedAt: o.CreatedAt}
}

func orgMemberToModel(m *OrgMember) *model.OrgMember {
	return &model.OrgMember{OrgID: m.OrgID, UserID: m.UserID, Role: model.OrgRole(m.Role)}
}

func moduleToModel(m *Module) *model.Module {
	return &model.Module{
		ID: m.ID, OwnerName: m.OwnerName, OwnerType: model.OwnerType(m.OwnerType),
		Name: m.Name, Visibility: model.Visibility(m.Visibility), CreatedAt: m.CreatedAt,
	}
}

func moduleFromModel(m *model.Module) *Module {
	return &Module{
		ID: m.ID, OwnerName: m.OwnerName, OwnerType: string(m.OwnerType),
		Name: m.Name, Visibility: string(m.Visibility), CreatedAt: m.CreatedAt,
	}
}

func commitToModel(c *Commit) *model.Commit {
	return &model.Commit{
		ID: c.ID, ModuleID: c.ModuleID, StorageKey: c.StorageKey,
		Manifest: c.Manifest, CreatedByUserID: c.CreatedByUserID, CreatedAt: c.CreatedAt,
	}
}

func commitFromModel(c *model.Commit) *Commit {
	return &Commit{
		ID: c.ID, ModuleID: c.ModuleID, StorageKey: c.StorageKey,
		Manifest: c.Manifest, CreatedByUserID: c.CreatedByUserID, CreatedAt: c.CreatedAt,
	}
}

func labelToModel(l *Label) *model.Label {
	return &model.Label{
		ID: l.ID, ModuleID: l.ModuleID, Name: l.Name,
		CommitID: l.CommitID, CreatedAt: l.CreatedAt, UpdatedAt: l.UpdatedAt,
	}
}

func labelFromModel(l *model.Label) *Label {
	return &Label{
		ID: l.ID, ModuleID: l.ModuleID, Name: l.Name,
		CommitID: l.CommitID, CreatedAt: l.CreatedAt, UpdatedAt: l.UpdatedAt,
	}
}

func tokenToModel(t *Token) *model.Token {
	return &model.Token{
		ID: t.ID, UserID: t.UserID, Hash: t.Hash,
		Note: t.Note, ExpiresAt: t.ExpiresAt, CreatedAt: t.CreatedAt,
	}
}

func tokenFromModel(t *model.Token) *Token {
	return &Token{
		ID: t.ID, UserID: t.UserID, Hash: t.Hash,
		Note: t.Note, ExpiresAt: t.ExpiresAt, CreatedAt: t.CreatedAt,
	}
}
