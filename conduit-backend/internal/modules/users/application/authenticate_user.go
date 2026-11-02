package application

import (
	"context"

	"conduit/backend/internal/modules/users/domain"
	sharedomain "conduit/backend/internal/shared/domain"
)

const timingHash = "$2b$10$la9MsO03GrXB6wPVIxJHf.AWuYIX2h8xZbkFpc8VuCbg2atOt7OEe"

type AuthenticateUserInput struct {
	Email    sharedomain.Raw
	Password sharedomain.Raw
}

type AuthenticateUser struct {
	users  domain.Repository
	hasher domain.PasswordHasher
	tokens domain.TokenService
}

func NewAuthenticateUser(users domain.Repository, hasher domain.PasswordHasher, tokens domain.TokenService) *AuthenticateUser {
	return &AuthenticateUser{users: users, hasher: hasher, tokens: tokens}
}

func (uc *AuthenticateUser) Execute(ctx context.Context, input AuthenticateUserInput) (UserDTO, error) {
	email, emailOK := input.Email.Text()
	password, passwordOK := input.Password.Text()
	errs := sharedomain.NewErrorCollector()
	if !emailOK || sharedomain.TrimText(email) == "" {
		errs.Add("email", sharedomain.Blank)
	}
	if !passwordOK || password == "" {
		errs.Add("password", sharedomain.Blank)
	}
	if err := errs.Err(); err != nil {
		return UserDTO{}, err
	}

	user, found, err := uc.users.FindByEmail(ctx, sharedomain.TrimText(email))
	if err != nil {
		return UserDTO{}, err
	}
	hash := timingHash
	if found {
		hash = user.PasswordHash()
	}
	matches := uc.hasher.Compare(ctx, password, hash)
	if !found || !matches {
		return UserDTO{}, sharedomain.NewInvalidCredentialsError()
	}

	token, err := uc.tokens.Issue(user.ID())
	if err != nil {
		return UserDTO{}, err
	}
	return ToUserDTO(user, token), nil
}
