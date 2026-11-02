package domain

import "context"

type TakenQuery struct {
	Email     string
	Username  string
	ExcludeID string
}

type Repository interface {
	NextID() string
	Add(ctx context.Context, user User) error
	Update(ctx context.Context, user User) (bool, error)
	FindByID(ctx context.Context, id string) (User, bool, error)
	FindByIDs(ctx context.Context, ids []string) ([]User, error)
	FindByEmail(ctx context.Context, email string) (User, bool, error)
	FindByUsername(ctx context.Context, username string) (User, bool, error)
	ExistsByID(ctx context.Context, id string) (bool, error)
	FindTakenFields(ctx context.Context, query TakenQuery) ([]string, error)
}

type FollowRepository interface {
	Follow(ctx context.Context, followerID, followedID string) error
	Unfollow(ctx context.Context, followerID, followedID string) error
	IsFollowing(ctx context.Context, followerID, followedID string) (bool, error)
	FindFollowedAmong(ctx context.Context, followerID string, candidateIDs []string) (map[string]bool, error)
	ListFollowedIDs(ctx context.Context, followerID string) ([]string, error)
}
