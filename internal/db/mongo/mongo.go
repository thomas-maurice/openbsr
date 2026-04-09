package mongo

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/thomas-maurice/openbsr/internal/db/iface"
	"github.com/thomas-maurice/openbsr/internal/model"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)



type DB struct {
	client *mongo.Client
	db     *mongo.Database
}

func Connect(ctx context.Context, uri string) (*DB, error) {
	client, err := mongo.Connect(options.Client().ApplyURI(uri))
	if err != nil {
		return nil, err
	}
	ctx2, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := client.Ping(ctx2, nil); err != nil {
		return nil, err
	}
	return &DB{client: client, db: client.Database("bsr")}, nil
}

func (d *DB) Close(ctx context.Context) error {
	return d.client.Disconnect(ctx)
}

func (d *DB) Repos() *iface.Repos {
	return &iface.Repos{
		Users:   &UserStore{col: d.db.Collection("users"), members: d.db.Collection("org_members")},
		Orgs:    &OrgStore{col: d.db.Collection("orgs"), members: d.db.Collection("org_members")},
		Modules: &ModuleStore{col: d.db.Collection("modules")},
		Commits: &CommitStore{col: d.db.Collection("commits")},
		Labels:  &LabelStore{col: d.db.Collection("labels")},
		Tokens:  &TokenStore{col: d.db.Collection("tokens")},
	}
}

// --- UserStore ---

type UserStore struct {
	col     *mongo.Collection
	members *mongo.Collection
}

func (s *UserStore) Create(ctx context.Context, u *model.User) error {
	_, err := s.col.InsertOne(ctx, u)
	return err
}

func (s *UserStore) GetByUsername(ctx context.Context, username string) (*model.User, error) {
	var u model.User
	err := s.col.FindOne(ctx, bson.M{"username": username}).Decode(&u)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, iface.ErrNotFound
	}
	return &u, err
}

func (s *UserStore) GetByID(ctx context.Context, id string) (*model.User, error) {
	var u model.User
	err := s.col.FindOne(ctx, bson.M{"id": id}).Decode(&u)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, iface.ErrNotFound
	}
	return &u, err
}

func (s *UserStore) ListByOrg(ctx context.Context, orgID string) ([]*model.User, error) {
	cursor, err := s.members.Find(ctx, bson.M{"orgid": orgID})
	if err != nil {
		return nil, err
	}
	var members []*model.OrgMember
	if err := cursor.All(ctx, &members); err != nil {
		return nil, err
	}
	var users []*model.User
	for _, m := range members {
		var u model.User
		err := s.col.FindOne(ctx, bson.M{"id": m.UserID}).Decode(&u)
		if err != nil {
			continue
		}
		users = append(users, &u)
	}
	return users, nil
}

// --- TokenStore ---

type TokenStore struct {
	col *mongo.Collection
}

func (s *TokenStore) Create(ctx context.Context, t *model.Token) error {
	_, err := s.col.InsertOne(ctx, t)
	return err
}

func (s *TokenStore) GetByHash(ctx context.Context, hash string) (*model.Token, error) {
	var t model.Token
	err := s.col.FindOne(ctx, bson.M{"hash": hash}).Decode(&t)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, iface.ErrNotFound
	}
	return &t, err
}

func (s *TokenStore) ListByUser(ctx context.Context, userID string) ([]*model.Token, error) {
	cursor, err := s.col.Find(ctx, bson.M{"userid": userID})
	if err != nil {
		return nil, err
	}
	var tokens []*model.Token
	if err := cursor.All(ctx, &tokens); err != nil {
		return nil, err
	}
	return tokens, nil
}

func (s *TokenStore) DeleteExpired(ctx context.Context) error {
	_, err := s.col.DeleteMany(ctx, bson.M{"expiresat": bson.M{"$lt": time.Now().UTC()}})
	return err
}

func (s *TokenStore) DeleteByUserAndNote(ctx context.Context, userID, note string) error {
	_, err := s.col.DeleteMany(ctx, bson.M{"userid": userID, "note": note})
	return err
}

func (s *TokenStore) Revoke(ctx context.Context, id, userID string) error {
	res, err := s.col.DeleteOne(ctx, bson.M{"id": id, "userid": userID})
	if err != nil {
		return err
	}
	if res.DeletedCount == 0 {
		return iface.ErrNotFound
	}
	return nil
}

// --- OrgStore (stub for M1) ---

type OrgStore struct {
	col     *mongo.Collection
	members *mongo.Collection
}

func (s *OrgStore) Create(ctx context.Context, o *model.Org) error {
	_, err := s.col.InsertOne(ctx, o)
	return err
}

func (s *OrgStore) GetByName(ctx context.Context, name string) (*model.Org, error) {
	var o model.Org
	err := s.col.FindOne(ctx, bson.M{"name": name}).Decode(&o)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, iface.ErrNotFound
	}
	return &o, err
}

func (s *OrgStore) GetByID(ctx context.Context, id string) (*model.Org, error) {
	var o model.Org
	err := s.col.FindOne(ctx, bson.M{"id": id}).Decode(&o)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, iface.ErrNotFound
	}
	return &o, err
}

func (s *OrgStore) AddMember(ctx context.Context, orgID, userID string, role model.OrgRole) error {
	_, err := s.members.InsertOne(ctx, model.OrgMember{OrgID: orgID, UserID: userID, Role: role})
	return err
}

func (s *OrgStore) RemoveMember(ctx context.Context, orgID, userID string) error {
	_, err := s.members.DeleteOne(ctx, bson.M{"orgid": orgID, "userid": userID})
	return err
}

func (s *OrgStore) ListMembers(ctx context.Context, orgID string) ([]*model.OrgMember, error) {
	cursor, err := s.members.Find(ctx, bson.M{"orgid": orgID})
	if err != nil {
		return nil, err
	}
	var members []*model.OrgMember
	if err := cursor.All(ctx, &members); err != nil {
		return nil, err
	}
	return members, nil
}

func (s *OrgStore) GetMember(ctx context.Context, orgID, userID string) (*model.OrgMember, error) {
	var m model.OrgMember
	err := s.members.FindOne(ctx, bson.M{"orgid": orgID, "userid": userID}).Decode(&m)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, iface.ErrNotFound
	}
	return &m, err
}

// --- ModuleStore (stub for M1) ---

type ModuleStore struct {
	col *mongo.Collection
}

func (s *ModuleStore) Create(ctx context.Context, m *model.Module) error {
	_, err := s.col.InsertOne(ctx, m)
	return err
}

func (s *ModuleStore) Get(ctx context.Context, ownerName, repoName string) (*model.Module, error) {
	var m model.Module
	err := s.col.FindOne(ctx, bson.M{"ownername": ownerName, "name": repoName}).Decode(&m)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, iface.ErrNotFound
	}
	return &m, err
}

func (s *ModuleStore) GetByID(ctx context.Context, id string) (*model.Module, error) {
	var m model.Module
	err := s.col.FindOne(ctx, bson.M{"id": id}).Decode(&m)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, iface.ErrNotFound
	}
	return &m, err
}

func (s *ModuleStore) List(ctx context.Context, ownerName string) ([]*model.Module, error) {
	cursor, err := s.col.Find(ctx, bson.M{"ownername": ownerName})
	if err != nil {
		return nil, err
	}
	var modules []*model.Module
	if err := cursor.All(ctx, &modules); err != nil {
		return nil, err
	}
	return modules, nil
}

func escapeRegex(s string) string {
	special := `\.+*?^$()[]{}|`
	result := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		if strings.ContainsRune(special, rune(s[i])) {
			result = append(result, '\\')
		}
		result = append(result, s[i])
	}
	return string(result)
}

func (s *ModuleStore) ListPublic(ctx context.Context, query string, limit int) ([]*model.Module, error) {
	filter := bson.M{"visibility": "public"}
	if query != "" {
		escaped := escapeRegex(query)
		filter["$or"] = []bson.M{
			{"name": bson.M{"$regex": escaped, "$options": "i"}},
			{"ownername": bson.M{"$regex": escaped, "$options": "i"}},
		}
	}
	opts := options.Find().SetLimit(int64(limit)).SetSort(bson.M{"createdat": -1})
	cursor, err := s.col.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	var modules []*model.Module
	if err := cursor.All(ctx, &modules); err != nil {
		return nil, err
	}
	return modules, nil
}

func (s *ModuleStore) SetVisibility(ctx context.Context, id string, vis model.Visibility) error {
	_, err := s.col.UpdateOne(ctx, bson.M{"id": id}, bson.M{"$set": bson.M{"visibility": vis}})
	return err
}

func (s *ModuleStore) Delete(ctx context.Context, id string) error {
	_, err := s.col.DeleteOne(ctx, bson.M{"id": id})
	return err
}

// --- CommitStore (stub for M1) ---

type CommitStore struct {
	col *mongo.Collection
}

func (s *CommitStore) Create(ctx context.Context, c *model.Commit) error {
	_, err := s.col.InsertOne(ctx, c)
	return err
}

func (s *CommitStore) GetByID(ctx context.Context, moduleID, commitID string) (*model.Commit, error) {
	var c model.Commit
	err := s.col.FindOne(ctx, bson.M{"moduleid": moduleID, "id": commitID}).Decode(&c)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, iface.ErrNotFound
	}
	return &c, err
}

func (s *CommitStore) GetLatest(ctx context.Context, moduleID string) (*model.Commit, error) {
	opts := options.FindOne().SetSort(bson.M{"createdat": -1})
	var c model.Commit
	err := s.col.FindOne(ctx, bson.M{"moduleid": moduleID}, opts).Decode(&c)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, iface.ErrNotFound
	}
	return &c, err
}

func (s *CommitStore) ListByModule(ctx context.Context, moduleID string) ([]*model.Commit, error) {
	opts := options.Find().SetSort(bson.M{"createdat": -1})
	cursor, err := s.col.Find(ctx, bson.M{"moduleid": moduleID}, opts)
	if err != nil {
		return nil, err
	}
	var commits []*model.Commit
	if err := cursor.All(ctx, &commits); err != nil {
		return nil, err
	}
	return commits, nil
}

// --- LabelStore (stub for M1) ---

type LabelStore struct {
	col *mongo.Collection
}

func (s *LabelStore) Upsert(ctx context.Context, l *model.Label) error {
	filter := bson.M{"moduleid": l.ModuleID, "name": l.Name}
	update := bson.M{"$set": l}
	opts := options.UpdateOne().SetUpsert(true)
	_, err := s.col.UpdateOne(ctx, filter, update, opts)
	return err
}

func (s *LabelStore) Get(ctx context.Context, moduleID, name string) (*model.Label, error) {
	var l model.Label
	err := s.col.FindOne(ctx, bson.M{"moduleid": moduleID, "name": name}).Decode(&l)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, iface.ErrNotFound
	}
	return &l, err
}

func (s *LabelStore) List(ctx context.Context, moduleID string) ([]*model.Label, error) {
	cursor, err := s.col.Find(ctx, bson.M{"moduleid": moduleID})
	if err != nil {
		return nil, err
	}
	var labels []*model.Label
	if err := cursor.All(ctx, &labels); err != nil {
		return nil, err
	}
	return labels, nil
}
