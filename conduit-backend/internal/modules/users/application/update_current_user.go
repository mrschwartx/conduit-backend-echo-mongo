package application

import (
	"context"
	"time"

	"conduit/backend/internal/modules/users/domain"
	sharedomain "conduit/backend/internal/shared/domain"
)

type UserChanges struct {
	Username sharedomain.Raw
	Email    sharedomain.Raw
	Password sharedomain.Raw
	Bio      sharedomain.Raw
	Image    sharedomain.Raw
}

type UpdateCurrentUserInput struct {
	UserID  string
	Changes UserChanges
}

type UpdateCurrentUser struct {
	users  domain.Repository
	hasher domain.PasswordHasher
	tokens domain.TokenService
	now    func() time.Time
}

func NewUpdateCurrentUser(users domain.Repository, hasher domain.PasswordHasher, tokens domain.TokenService) *UpdateCurrentUser {
	return &UpdateCurrentUser{users: users, hasher: hasher, tokens: tokens, now: time.Now}
}

func (uc *UpdateCurrentUser) Execute(ctx context.Context, input UpdateCurrentUserInput) (UserDTO, error) {
	user, found, err := uc.users.FindByID(ctx, input.UserID)
	if err != nil {
		return UserDTO{}, err
	}
	if !found {
		return UserDTO{}, sharedomain.NewInvalidTokenError()
	}

	changes := input.Changes
	if !changes.Username.IsSet() && !changes.Email.IsSet() && !changes.Password.IsSet() &&
		!changes.Bio.IsSet() && !changes.Image.IsSet() {
		return UserDTO{}, sharedomain.NewFieldError("user", "can't be empty")
	}

	errs := sharedomain.NewErrorCollector()
	var username *domain.Username
	var email *domain.Email
	var password *domain.PlainPassword
	profile := domain.ProfileChanges{}

	if changes.Username.IsSet() {
		value, err := domain.NewUsername(changes.Username)
		if err := errs.Capture(err); err != nil {
			return UserDTO{}, err
		}
		if err == nil {
			username = &value
		}
	}
	if changes.Email.IsSet() {
		value, err := domain.NewEmail(changes.Email)
		if err := errs.Capture(err); err != nil {
			return UserDTO{}, err
		}
		if err == nil {
			email = &value
		}
	}
	if changes.Password.IsSet() {
		value, err := domain.NewPlainPassword(changes.Password)
		if err := errs.Capture(err); err != nil {
			return UserDTO{}, err
		}
		if err == nil {
			password = &value
		}
	}
	if changes.Bio.IsSet() {
		value, err := domain.NewBio(changes.Bio)
		if err := errs.Capture(err); err != nil {
			return UserDTO{}, err
		}
		if err == nil {
			profile.SetBio = true
			profile.Bio = value
		}
	}
	if changes.Image.IsSet() {
		value, err := domain.NewProfileImage(changes.Image)
		if err := errs.Capture(err); err != nil {
			return UserDTO{}, err
		}
		if err == nil {
			profile.SetImage = true
			profile.Image = value
		}
	}
	if err := errs.Err(); err != nil {
		return UserDTO{}, err
	}

	if username != nil || email != nil {
		query := domain.TakenQuery{ExcludeID: user.ID()}
		if username != nil {
			query.Username = username.Value()
		}
		if email != nil {
			query.Email = email.Value()
		}
		taken, err := uc.users.FindTakenFields(ctx, query)
		if err != nil {
			return UserDTO{}, err
		}
		if len(taken) > 0 {
			return UserDTO{}, sharedomain.NewConflictError(taken...)
		}
	}

	if password != nil {
		hashed, err := uc.hasher.Hash(ctx, password.Value())
		if err != nil {
			return UserDTO{}, err
		}
		profile.PasswordHash = &hashed
	}
	profile.Username = username
	profile.Email = email

	user.UpdateProfile(profile, uc.now())
	updated, err := uc.users.Update(ctx, user)
	if err != nil {
		return UserDTO{}, err
	}
	if !updated {
		return UserDTO{}, sharedomain.NewInvalidTokenError()
	}

	token, err := uc.tokens.Issue(user.ID())
	if err != nil {
		return UserDTO{}, err
	}
	return ToUserDTO(user, token), nil
}
