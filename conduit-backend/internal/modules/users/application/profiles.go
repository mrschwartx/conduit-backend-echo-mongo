package application

import (
	"context"

	"conduit/backend/internal/modules/users/domain"
	sharedomain "conduit/backend/internal/shared/domain"
)

const profileResource = "profile"

type GetProfile struct {
	users   domain.Repository
	follows domain.FollowRepository
}

func NewGetProfile(users domain.Repository, follows domain.FollowRepository) *GetProfile {
	return &GetProfile{users: users, follows: follows}
}

func (uc *GetProfile) Execute(ctx context.Context, username, viewerID string) (ProfileDTO, error) {
	user, found, err := uc.users.FindByUsername(ctx, username)
	if err != nil {
		return ProfileDTO{}, err
	}
	if !found {
		return ProfileDTO{}, sharedomain.NewNotFoundError(profileResource)
	}
	following := false
	if viewerID != "" {
		following, err = uc.follows.IsFollowing(ctx, viewerID, user.ID())
		if err != nil {
			return ProfileDTO{}, err
		}
	}
	return ToProfileDTO(user, following), nil
}

type FollowUser struct {
	users   domain.Repository
	follows domain.FollowRepository
}

func NewFollowUser(users domain.Repository, follows domain.FollowRepository) *FollowUser {
	return &FollowUser{users: users, follows: follows}
}

func (uc *FollowUser) Execute(ctx context.Context, followerID, username string) (ProfileDTO, error) {
	target, found, err := uc.users.FindByUsername(ctx, username)
	if err != nil {
		return ProfileDTO{}, err
	}
	if !found {
		return ProfileDTO{}, sharedomain.NewNotFoundError(profileResource)
	}
	if target.ID() == followerID {
		return ProfileDTO{}, sharedomain.NewFieldError("username", "you cannot follow yourself")
	}
	if err := uc.follows.Follow(ctx, followerID, target.ID()); err != nil {
		return ProfileDTO{}, err
	}
	return ToProfileDTO(target, true), nil
}

type UnfollowUser struct {
	users   domain.Repository
	follows domain.FollowRepository
}

func NewUnfollowUser(users domain.Repository, follows domain.FollowRepository) *UnfollowUser {
	return &UnfollowUser{users: users, follows: follows}
}

func (uc *UnfollowUser) Execute(ctx context.Context, followerID, username string) (ProfileDTO, error) {
	target, found, err := uc.users.FindByUsername(ctx, username)
	if err != nil {
		return ProfileDTO{}, err
	}
	if !found {
		return ProfileDTO{}, sharedomain.NewNotFoundError(profileResource)
	}
	if err := uc.follows.Unfollow(ctx, followerID, target.ID()); err != nil {
		return ProfileDTO{}, err
	}
	return ToProfileDTO(target, false), nil
}
