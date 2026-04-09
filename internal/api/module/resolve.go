package module

import (
	"context"
	"errors"

	modulev1 "buf.build/gen/go/bufbuild/registry/protocolbuffers/go/buf/registry/module/v1"
	"github.com/thomas-maurice/openbsr/internal/db/iface"
	"github.com/thomas-maurice/openbsr/internal/model"
)

// resolveResourceRef resolves a ResourceRef to a module and optionally a commit.
// Returns (module, commit, error). If the ref points to a module only, commit is nil.
func resolveResourceRef(ctx context.Context, repos *iface.Repos, ref *modulev1.ResourceRef) (*model.Module, *model.Commit, error) {
	if ref == nil {
		return nil, nil, errors.New("resource ref required")
	}

	// By ID — try commit first, then module
	if ref.GetId() != "" {
		// Try as commit ID across all modules (simplified: search commits table)
		// For now, ID-based lookup is not practical without a global commits index.
		// We'll handle this case when we have a commit ID.
		return nil, nil, errors.New("ID-based resource ref not yet supported")
	}

	name := ref.GetName()
	if name == nil {
		return nil, nil, errors.New("resource ref must have name or id")
	}

	// Resolve the module
	m, err := repos.Modules.Get(ctx, name.GetOwner(), name.GetModule())
	if err != nil {
		if errors.Is(err, iface.ErrNotFound) {
			return nil, nil, errors.New("module not found: " + name.GetOwner() + "/" + name.GetModule())
		}
		return nil, nil, err
	}

	// If ref has a label name child, resolve to that label's commit
	if name.GetLabelName() != "" {
		label, err := repos.Labels.Get(ctx, m.ID, name.GetLabelName())
		if err != nil {
			if errors.Is(err, iface.ErrNotFound) {
				return nil, nil, errors.New("label not found: " + name.GetLabelName())
			}
			return nil, nil, err
		}
		commit, err := repos.Commits.GetByID(ctx, m.ID, label.CommitID)
		if err != nil {
			return nil, nil, err
		}
		return m, commit, nil
	}

	// If ref has a "ref" child, try as commit ID first, then label name
	if name.GetRef() != "" {
		// Try as commit ID
		commit, err := repos.Commits.GetByID(ctx, m.ID, name.GetRef())
		if err == nil {
			return m, commit, nil
		}
		// Try as label name
		label, err := repos.Labels.Get(ctx, m.ID, name.GetRef())
		if err == nil {
			commit, err := repos.Commits.GetByID(ctx, m.ID, label.CommitID)
			if err != nil {
				return nil, nil, err
			}
			return m, commit, nil
		}
		return nil, nil, errors.New("ref not found: " + name.GetRef())
	}

	// No child — resolve to default label "main"
	label, err := repos.Labels.Get(ctx, m.ID, "main")
	if err != nil {
		if errors.Is(err, iface.ErrNotFound) {
			// Module exists but has no commits yet
			return m, nil, nil
		}
		return nil, nil, err
	}
	commit, err := repos.Commits.GetByID(ctx, m.ID, label.CommitID)
	if err != nil {
		return nil, nil, err
	}
	return m, commit, nil
}
