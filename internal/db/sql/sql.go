package sql

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/thomas-maurice/openbsr/internal/db/iface"
	"github.com/thomas-maurice/openbsr/internal/model"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/logger"
)

type DB struct {
	db    *gorm.DB
	sqlDB *sql.DB
}

func Connect(driver, dsn string) (*DB, error) {
	var dialector gorm.Dialector
	switch driver {
	case "postgres":
		dialector = postgres.Open(dsn)
	case "sqlite":
		dialector = sqlite.Open(dsn + "?_foreign_keys=on")
	default:
		return nil, errors.New("unsupported sql driver: " + driver)
	}

	db, err := gorm.Open(dialector, &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return nil, err
	}

	if err := db.AutoMigrate(&User{}, &Org{}, &OrgMember{}, &Module{}, &Commit{}, &Label{}, &Token{}); err != nil {
		return nil, err
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}

	return &DB{db: db, sqlDB: sqlDB}, nil
}

// GormDB returns the underlying GORM handle, useful for sharing with
// the database-backed blob store (storage.DBStore).
func (d *DB) GormDB() *gorm.DB { return d.db }

func (d *DB) Close() error {
	return d.sqlDB.Close()
}

func (d *DB) Repos() *iface.Repos {
	return &iface.Repos{
		Users:   &UserStore{db: d.db},
		Orgs:    &OrgStore{db: d.db},
		Modules: &ModuleStore{db: d.db},
		Commits: &CommitStore{db: d.db},
		Labels:  &LabelStore{db: d.db},
		Tokens:  &TokenStore{db: d.db},
	}
}

func mapNotFound(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return iface.ErrNotFound
	}
	return err
}

// ===================== UserStore =====================

type UserStore struct{ db *gorm.DB }

func (s *UserStore) Create(_ context.Context, u *model.User) error {
	return s.db.Create(userFromModel(u)).Error
}

func (s *UserStore) GetByUsername(_ context.Context, username string) (*model.User, error) {
	var u User
	if err := s.db.Where("username = ?", username).First(&u).Error; err != nil {
		return nil, mapNotFound(err)
	}
	return userToModel(&u), nil
}

func (s *UserStore) GetByID(_ context.Context, id string) (*model.User, error) {
	var u User
	if err := s.db.Where("id = ?", id).First(&u).Error; err != nil {
		return nil, mapNotFound(err)
	}
	return userToModel(&u), nil
}

func (s *UserStore) ListByOrg(_ context.Context, orgID string) ([]*model.User, error) {
	var users []User
	err := s.db.Joins("JOIN org_members ON org_members.user_id = users.id").
		Where("org_members.org_id = ?", orgID).Find(&users).Error
	if err != nil {
		return nil, err
	}
	result := make([]*model.User, len(users))
	for i := range users {
		result[i] = userToModel(&users[i])
	}
	return result, nil
}

// ===================== OrgStore =====================

type OrgStore struct{ db *gorm.DB }

func (s *OrgStore) Create(_ context.Context, o *model.Org) error {
	return s.db.Create(orgFromModel(o)).Error
}

func (s *OrgStore) GetByName(_ context.Context, name string) (*model.Org, error) {
	var o Org
	if err := s.db.Where("name = ?", name).First(&o).Error; err != nil {
		return nil, mapNotFound(err)
	}
	return orgToModel(&o), nil
}

func (s *OrgStore) GetByID(_ context.Context, id string) (*model.Org, error) {
	var o Org
	if err := s.db.Where("id = ?", id).First(&o).Error; err != nil {
		return nil, mapNotFound(err)
	}
	return orgToModel(&o), nil
}

func (s *OrgStore) AddMember(_ context.Context, orgID, userID string, role model.OrgRole) error {
	return s.db.Create(&OrgMember{OrgID: orgID, UserID: userID, Role: string(role)}).Error
}

func (s *OrgStore) RemoveMember(_ context.Context, orgID, userID string) error {
	return s.db.Where("org_id = ? AND user_id = ?", orgID, userID).Delete(&OrgMember{}).Error
}

func (s *OrgStore) ListMembers(_ context.Context, orgID string) ([]*model.OrgMember, error) {
	var members []OrgMember
	if err := s.db.Where("org_id = ?", orgID).Find(&members).Error; err != nil {
		return nil, err
	}
	result := make([]*model.OrgMember, len(members))
	for i := range members {
		result[i] = orgMemberToModel(&members[i])
	}
	return result, nil
}

func (s *OrgStore) GetMember(_ context.Context, orgID, userID string) (*model.OrgMember, error) {
	var m OrgMember
	if err := s.db.Where("org_id = ? AND user_id = ?", orgID, userID).First(&m).Error; err != nil {
		return nil, mapNotFound(err)
	}
	return orgMemberToModel(&m), nil
}

// ===================== ModuleStore =====================

type ModuleStore struct{ db *gorm.DB }

func (s *ModuleStore) Create(_ context.Context, m *model.Module) error {
	return s.db.Create(moduleFromModel(m)).Error
}

func (s *ModuleStore) Get(_ context.Context, ownerName, repoName string) (*model.Module, error) {
	var m Module
	if err := s.db.Where("owner_name = ? AND name = ?", ownerName, repoName).First(&m).Error; err != nil {
		return nil, mapNotFound(err)
	}
	return moduleToModel(&m), nil
}

func (s *ModuleStore) GetByID(_ context.Context, id string) (*model.Module, error) {
	var m Module
	if err := s.db.Where("id = ?", id).First(&m).Error; err != nil {
		return nil, mapNotFound(err)
	}
	return moduleToModel(&m), nil
}

func (s *ModuleStore) List(_ context.Context, ownerName string) ([]*model.Module, error) {
	var modules []Module
	if err := s.db.Where("owner_name = ?", ownerName).Find(&modules).Error; err != nil {
		return nil, err
	}
	result := make([]*model.Module, len(modules))
	for i := range modules {
		result[i] = moduleToModel(&modules[i])
	}
	return result, nil
}

func (s *ModuleStore) ListPublic(_ context.Context, query string, limit int) ([]*model.Module, error) {
	tx := s.db.Where("visibility = ?", "public")
	if query != "" {
		q := "%" + strings.ToLower(query) + "%"
		tx = tx.Where("LOWER(name) LIKE ? OR LOWER(owner_name) LIKE ?", q, q)
	}
	var modules []Module
	if err := tx.Order("created_at DESC").Limit(limit).Find(&modules).Error; err != nil {
		return nil, err
	}
	result := make([]*model.Module, len(modules))
	for i := range modules {
		result[i] = moduleToModel(&modules[i])
	}
	return result, nil
}

func (s *ModuleStore) SetVisibility(_ context.Context, id string, vis model.Visibility) error {
	return s.db.Model(&Module{}).Where("id = ?", id).Update("visibility", string(vis)).Error
}

func (s *ModuleStore) Delete(_ context.Context, id string) error {
	return s.db.Where("id = ?", id).Delete(&Module{}).Error
}

// ===================== CommitStore =====================

type CommitStore struct{ db *gorm.DB }

func (s *CommitStore) Create(_ context.Context, c *model.Commit) error {
	return s.db.Create(commitFromModel(c)).Error
}

func (s *CommitStore) GetByID(_ context.Context, moduleID, commitID string) (*model.Commit, error) {
	var c Commit
	if err := s.db.Where("module_id = ? AND id = ?", moduleID, commitID).First(&c).Error; err != nil {
		return nil, mapNotFound(err)
	}
	return commitToModel(&c), nil
}

func (s *CommitStore) GetLatest(_ context.Context, moduleID string) (*model.Commit, error) {
	var c Commit
	if err := s.db.Where("module_id = ?", moduleID).Order("created_at DESC").First(&c).Error; err != nil {
		return nil, mapNotFound(err)
	}
	return commitToModel(&c), nil
}

func (s *CommitStore) ListByModule(_ context.Context, moduleID string) ([]*model.Commit, error) {
	var commits []Commit
	if err := s.db.Where("module_id = ?", moduleID).Order("created_at DESC").Find(&commits).Error; err != nil {
		return nil, err
	}
	result := make([]*model.Commit, len(commits))
	for i := range commits {
		result[i] = commitToModel(&commits[i])
	}
	return result, nil
}

// ===================== LabelStore =====================

type LabelStore struct{ db *gorm.DB }

func (s *LabelStore) Upsert(_ context.Context, l *model.Label) error {
	gl := labelFromModel(l)
	return s.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "module_id"}, {Name: "name"}},
		DoUpdates: clause.AssignmentColumns([]string{"commit_id", "updated_at"}),
	}).Create(gl).Error
}

func (s *LabelStore) Get(_ context.Context, moduleID, name string) (*model.Label, error) {
	var l Label
	if err := s.db.Where("module_id = ? AND name = ?", moduleID, name).First(&l).Error; err != nil {
		return nil, mapNotFound(err)
	}
	return labelToModel(&l), nil
}

func (s *LabelStore) List(_ context.Context, moduleID string) ([]*model.Label, error) {
	var labels []Label
	if err := s.db.Where("module_id = ?", moduleID).Find(&labels).Error; err != nil {
		return nil, err
	}
	result := make([]*model.Label, len(labels))
	for i := range labels {
		result[i] = labelToModel(&labels[i])
	}
	return result, nil
}

// ===================== TokenStore =====================

type TokenStore struct{ db *gorm.DB }

func (s *TokenStore) Create(_ context.Context, t *model.Token) error {
	return s.db.Create(tokenFromModel(t)).Error
}

func (s *TokenStore) GetByHash(_ context.Context, hash string) (*model.Token, error) {
	var t Token
	if err := s.db.Where("hash = ?", hash).First(&t).Error; err != nil {
		return nil, mapNotFound(err)
	}
	return tokenToModel(&t), nil
}

func (s *TokenStore) ListByUser(_ context.Context, userID string) ([]*model.Token, error) {
	var tokens []Token
	if err := s.db.Where("user_id = ?", userID).Find(&tokens).Error; err != nil {
		return nil, err
	}
	result := make([]*model.Token, len(tokens))
	for i := range tokens {
		result[i] = tokenToModel(&tokens[i])
	}
	return result, nil
}

func (s *TokenStore) Revoke(_ context.Context, id, userID string) error {
	res := s.db.Where("id = ? AND user_id = ?", id, userID).Delete(&Token{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return iface.ErrNotFound
	}
	return nil
}

func (s *TokenStore) DeleteExpired(_ context.Context) error {
	return s.db.Where("expires_at < ?", time.Now().UTC()).Delete(&Token{}).Error
}

func (s *TokenStore) DeleteByUserAndNote(_ context.Context, userID, note string) error {
	return s.db.Where("user_id = ? AND note = ?", userID, note).Delete(&Token{}).Error
}
