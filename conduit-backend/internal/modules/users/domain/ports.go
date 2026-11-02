package domain

import "context"

type PasswordHasher interface {
	Hash(ctx context.Context, plain string) (string, error)
	Compare(ctx context.Context, plain, hash string) bool
}

type TokenService interface {
	Issue(userID string) (string, error)
	Verify(token string) (string, error)
}
