// Package authz provides centralized authorization using Casbin.
//
// The permission model is defined in model.conf and policy.csv (embedded).
// Role resolution is dynamic — the Authorizer queries the database to determine
// a user's role for a given resource, then asks Casbin whether that role permits
// the requested action.
//
// Roles:
//   - owner:      User owns the resource (user-owned module, or the user themselves)
//   - org_admin:  User is an admin member of the organization that owns the resource
//   - org_member: User is a regular member of the organization
//   - public:     No authentication — used for public module reads
//
// Resources:
//   - module: A protobuf module (read, write, admin, create)
//   - org:    An organization (manage_members)
//
// Actions:
//   - read:           View module content (commits, labels, files, FDS)
//   - write:          Push content to a module (upload)
//   - admin:          Manage labels, delete modules
//   - create:         Create new modules under an owner
//   - manage_members: Add/remove organization members
package authz

import (
	"context"
	_ "embed"
	"errors"
	"log/slog"
	"strings"

	"github.com/casbin/casbin/v2"
	"github.com/casbin/casbin/v2/model"
	stringadapter "github.com/casbin/casbin/v2/persist/string-adapter"
	"github.com/thomas-maurice/openbsr/internal/db/iface"
	modelPkg "github.com/thomas-maurice/openbsr/internal/model"
)

//go:embed model.conf
var modelConf string

//go:embed policy.csv
var policyCSV string

// Authorizer provides centralized authorization checks backed by Casbin.
type Authorizer struct {
	enforcer *casbin.Enforcer
	repos    *iface.Repos
}

// New creates a new Authorizer with the embedded Casbin model and policies.
func New(repos *iface.Repos) (*Authorizer, error) {
	m, err := model.NewModelFromString(modelConf)
	if err != nil {
		return nil, err
	}

	adapter := stringadapter.NewAdapter(policyCSV)

	e, err := casbin.NewEnforcer(m, adapter)
	if err != nil {
		return nil, err
	}

	return &Authorizer{enforcer: e, repos: repos}, nil
}

// resolveRole determines the user's role for a module.
// Returns: "owner", "org_admin", "org_member", or "public".
func (a *Authorizer) resolveModuleRole(ctx context.Context, userID string, m *modelPkg.Module) string {
	if userID == "" {
		return "public"
	}

	if m.OwnerType == modelPkg.OwnerTypeUser {
		// Check if user is the owner
		u, err := a.repos.Users.GetByID(ctx, userID)
		if err == nil && u.Username == m.OwnerName {
			return "owner"
		}
		return "public"
	}

	// Org-owned module — check membership
	o, err := a.repos.Orgs.GetByName(ctx, m.OwnerName)
	if err != nil {
		return "public"
	}
	member, err := a.repos.Orgs.GetMember(ctx, o.ID, userID)
	if err != nil {
		return "public"
	}
	if member.Role == modelPkg.OrgRoleAdmin {
		return "org_admin"
	}
	return "org_member"
}

// resolveOrgRole determines the user's role in an org.
func (a *Authorizer) resolveOrgRole(ctx context.Context, userID, orgID string) string {
	if userID == "" {
		return "public"
	}
	member, err := a.repos.Orgs.GetMember(ctx, orgID, userID)
	if err != nil {
		return "public"
	}
	if member.Role == modelPkg.OrgRoleAdmin {
		return "org_admin"
	}
	return "org_member"
}

// resolveOwnerRole determines the user's role for creating resources under an owner name.
func (a *Authorizer) resolveOwnerRole(ctx context.Context, userID, ownerName string) (string, error) {
	if userID == "" {
		return "public", nil
	}

	// Check if owner is the user themselves
	u, err := a.repos.Users.GetByID(ctx, userID)
	if err == nil && u.Username == ownerName {
		return "owner", nil
	}

	// Check if owner is an org
	o, err := a.repos.Orgs.GetByName(ctx, ownerName)
	if err != nil {
		if errors.Is(err, iface.ErrNotFound) {
			return "public", nil
		}
		return "", err
	}
	member, err := a.repos.Orgs.GetMember(ctx, o.ID, userID)
	if err != nil {
		return "public", nil
	}
	if member.Role == modelPkg.OrgRoleAdmin {
		return "org_admin", nil
	}
	return "org_member", nil
}

func (a *Authorizer) enforce(role, resource, action string) bool {
	ok, err := a.enforcer.Enforce(role, resource, action)
	if err != nil {
		slog.Error("casbin enforce error", "role", role, "resource", resource, "action", action, "err", err)
		return false
	}
	return ok
}

// --- Public API: typed authorization checks ---

// CanReadModule checks if the user can read a module's content.
// Public modules are readable by anyone. Private modules require ownership or membership.
func (a *Authorizer) CanReadModule(ctx context.Context, userID string, m *modelPkg.Module) bool {
	if m.Visibility == modelPkg.VisibilityPublic {
		return true
	}
	role := a.resolveModuleRole(ctx, userID, m)
	return a.enforce(role, "module", "read")
}

// CanWriteModule checks if the user can push content to a module.
func (a *Authorizer) CanWriteModule(ctx context.Context, userID string, m *modelPkg.Module) bool {
	role := a.resolveModuleRole(ctx, userID, m)
	return a.enforce(role, "module", "write")
}

// CanAdminModule checks if the user can perform admin actions on a module (manage labels, delete).
func (a *Authorizer) CanAdminModule(ctx context.Context, userID string, m *modelPkg.Module) bool {
	role := a.resolveModuleRole(ctx, userID, m)
	return a.enforce(role, "module", "admin")
}

// CanCreateModule checks if the user can create modules under the given owner name.
func (a *Authorizer) CanCreateModule(ctx context.Context, userID, ownerName string) bool {
	role, err := a.resolveOwnerRole(ctx, userID, ownerName)
	if err != nil {
		return false
	}
	return a.enforce(role, "module", "create")
}

// CanManageOrgMembers checks if the user can add/remove members in an org.
func (a *Authorizer) CanManageOrgMembers(ctx context.Context, userID, orgID string) bool {
	role := a.resolveOrgRole(ctx, userID, orgID)
	return a.enforce(role, "org", "manage_members")
}

// PolicySummary returns a human-readable summary of the policy for documentation.
func PolicySummary() string {
	lines := strings.Split(strings.TrimSpace(policyCSV), "\n")
	var sb strings.Builder
	sb.WriteString("Authorization Policy:\n")
	for _, line := range lines {
		parts := strings.Split(line, ", ")
		if len(parts) == 4 && parts[0] == "p" {
			sb.WriteString("  " + parts[1] + " can " + parts[3] + " " + parts[2] + "\n")
		}
	}
	return sb.String()
}
