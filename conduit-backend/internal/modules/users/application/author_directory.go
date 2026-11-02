package application

import (
	"context"

	"conduit/backend/internal/modules/users/domain"
	sharedapp "conduit/backend/internal/shared/application"
)

type AuthorDirectory struct {
	users   domain.Repository
	follows domain.FollowRepository
}

func NewAuthorDirectory(users domain.Repository, follows domain.FollowRepository) *AuthorDirectory {
	return &AuthorDirectory{users: users, follows: follows}
}

func (d *AuthorDirectory) FindProfiles(ctx context.Context, userIDs []string, viewerID string) (map[string]sharedapp.AuthorProfile, error) {
	ids := uniqueIDs(userIDs)
	profiles := map[string]sharedapp.AuthorProfile{}
	if len(ids) == 0 {
		return profiles, nil
	}
	users, err := d.users.FindByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	followed := map[string]bool{}
	if viewerID != "" {
		followed, err = d.follows.FindFollowedAmong(ctx, viewerID, ids)
		if err != nil {
			return nil, err
		}
	}
	for _, user := range users {
		profiles[user.ID()] = ToProfileDTO(user, followed[user.ID()])
	}
	return profiles, nil
}

func (d *AuthorDirectory) FindIDByUsername(ctx context.Context, username string) (string, bool, error) {
	user, found, err := d.users.FindByUsername(ctx, username)
	if err != nil || !found {
		return "", false, err
	}
	return user.ID(), true, nil
}

func (d *AuthorDirectory) ListFollowedIDs(ctx context.Context, userID string) ([]string, error) {
	return d.follows.ListFollowedIDs(ctx, userID)
}

func uniqueIDs(ids []string) []string {
	seen := make(map[string]bool, len(ids))
	unique := make([]string, 0, len(ids))
	for _, id := range ids {
		if !seen[id] {
			seen[id] = true
			unique = append(unique, id)
		}
	}
	return unique
}
