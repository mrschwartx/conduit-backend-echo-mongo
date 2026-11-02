package application

import (
	"context"
	"time"

	"conduit/backend/internal/modules/users/domain"
	sharedomain "conduit/backend/internal/shared/domain"
)

type RegisterUserInput struct {
	Username sharedomain.Raw
	Email    sharedomain.Raw
	Password sharedomain.Raw
}

type RegisterUser struct {
	users  domain.Repository
	hasher domain.PasswordHasher
	tokens domain.TokenService
	now    func() time.Time
}

func NewRegisterUser(users domain.Repository, hasher domain.PasswordHasher, tokens domain.TokenService) *RegisterUser {
	return &RegisterUser{users: users, hasher: hasher, tokens: tokens, now: time.Now}
}

func (uc *RegisterUser) Execute(ctx context.Context, input RegisterUserInput) (UserDTO, error) {
	errs := sharedomain.NewErrorCollector()
	username, err := domain.NewUsername(input.Username)
	if err := errs.Capture(err); err != nil {
		return UserDTO{}, err
	}
	email, err := domain.NewEmail(input.Email)
	if err := errs.Capture(err); err != nil {
		return UserDTO{}, err
	}
	password, err := domain.NewPlainPassword(input.Password)
	if err := errs.Capture(err); err != nil {
		return UserDTO{}, err
	}
	if err := errs.Err(); err != nil {
		return UserDTO{}, err
	}

	taken, err := uc.users.FindTakenFields(ctx, domain.TakenQuery{Email: email.Value(), Username: username.Value()})
	if err != nil {
		return UserDTO{}, err
	}
	if len(taken) > 0 {
		return UserDTO{}, sharedomain.NewConflictError(taken...)
	}

	passwordHash, err := uc.hasher.Hash(ctx, password.Value())
	if err != nil {
		return UserDTO{}, err
	}
	user := domain.Register(domain.Registration{
		ID:           uc.users.NextID(),
		Username:     username,
		Email:        email,
		PasswordHash: passwordHash,
		Now:          uc.now(),
	})
	if err := uc.users.Add(ctx, user); err != nil {
		return UserDTO{}, err
	}

	token, err := uc.tokens.Issue(user.ID())
	if err != nil {
		return UserDTO{}, err
	}
	return ToUserDTO(user, token), nil
}
