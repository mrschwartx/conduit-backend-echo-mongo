package application

import (
	"context"

	"conduit/backend/internal/modules/users/domain"
	sharedomain "conduit/backend/internal/shared/domain"
)

type GetCurrentUser struct {
	users  domain.Repository
	tokens domain.TokenService
}

func NewGetCurrentUser(users domain.Repository, tokens domain.TokenService) *GetCurrentUser {
	return &GetCurrentUser{users: users, tokens: tokens}
}

func (uc *GetCurrentUser) Execute(ctx context.Context, userID string) (UserDTO, error) {
	user, found, err := uc.users.FindByID(ctx, userID)
	if err != nil {
		return UserDTO{}, err
	}
	if !found {
		return UserDTO{}, sharedomain.NewInvalidTokenError()
	}
	token, err := uc.tokens.Issue(user.ID())
	if err != nil {
		return UserDTO{}, err
	}
	return ToUserDTO(user, token), nil
}
