package authn

import (
	"context"
	"errors"

	"connectrpc.com/connect"
	registryv1alpha1 "buf.build/gen/go/bufbuild/buf/protocolbuffers/go/buf/alpha/registry/v1alpha1"
	"buf.build/gen/go/bufbuild/buf/connectrpc/go/buf/alpha/registry/v1alpha1/registryv1alpha1connect"
	"github.com/thomas-maurice/openbsr/internal/auth"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type AuthnService struct {
	registryv1alpha1connect.UnimplementedAuthnServiceHandler
}

func NewAuthnService() *AuthnService {
	return &AuthnService{}
}

func (s *AuthnService) GetCurrentUser(
	ctx context.Context,
	req *connect.Request[registryv1alpha1.GetCurrentUserRequest],
) (*connect.Response[registryv1alpha1.GetCurrentUserResponse], error) {
	u := auth.UserFromContext(ctx)
	if u == nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("not authenticated"))
	}
	resp := &registryv1alpha1.GetCurrentUserResponse{
		User: &registryv1alpha1.User{
			Id:         u.ID,
			Username:   u.Username,
			CreateTime: timestamppb.New(u.CreatedAt),
		},
	}
	return connect.NewResponse(resp), nil
}
