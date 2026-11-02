package application

import "context"

type AuthorProfile struct {
	Username  string  `json:"username"`
	Bio       *string `json:"bio"`
	Image     *string `json:"image"`
	Following bool    `json:"following"`
}

type AuthorDirectory interface {
	FindProfiles(ctx context.Context, userIDs []string, viewerID string) (map[string]AuthorProfile, error)
	FindIDByUsername(ctx context.Context, username string) (string, bool, error)
	ListFollowedIDs(ctx context.Context, userID string) ([]string, error)
}
