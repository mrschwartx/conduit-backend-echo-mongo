package application

import (
	"context"

	"conduit/backend/internal/modules/users/domain"
	sharedomain "conduit/backend/internal/shared/domain"
)

type VerifyAccessToken struct {
	tokens domain.TokenService
	users  domain.Repository
}

func NewVerifyAccessToken(tokens domain.TokenService, users domain.Repository) *VerifyAccessToken {
	return &VerifyAccessToken{tokens: tokens, users: users}
}

func (uc *VerifyAccessToken) Execute(ctx context.Context, token string) (string, error) {
	if token == "" {
		return "", sharedomain.NewMissingTokenError()
	}
	userID, err := uc.tokens.Verify(token)
	if err != nil {
		return "", err
	}
	exists, err := uc.users.ExistsByID(ctx, userID)
	if err != nil {
		return "", err
	}
	if !exists {
		return "", sharedomain.NewInvalidTokenError()
	}
	return userID, nil
}
