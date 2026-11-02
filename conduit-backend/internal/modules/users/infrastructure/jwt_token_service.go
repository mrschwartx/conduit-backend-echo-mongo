package infrastructure

import (
	"errors"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"

	sharedomain "conduit/backend/internal/shared/domain"
)

type JWTTokenService struct {
	secret []byte
	ttl    time.Duration
	now    func() time.Time
}

func NewJWTTokenService(secret string, ttl time.Duration) *JWTTokenService {
	return &JWTTokenService{secret: []byte(secret), ttl: ttl, now: time.Now}
}

func (s *JWTTokenService) Issue(userID string) (string, error) {
	issuedAt := s.now()
	claims := jwt.RegisteredClaims{
		Subject:   userID,
		IssuedAt:  jwt.NewNumericDate(issuedAt),
		ExpiresAt: jwt.NewNumericDate(issuedAt.Add(s.ttl)),
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(s.secret)
}

func (s *JWTTokenService) Verify(token string) (string, error) {
	claims := &jwt.RegisteredClaims{}
	_, err := jwt.ParseWithClaims(
		token,
		claims,
		func(*jwt.Token) (any, error) { return s.secret, nil },
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithExpirationRequired(),
		jwt.WithTimeFunc(s.now),
	)
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return "", sharedomain.NewExpiredTokenError()
		}
		return "", sharedomain.NewInvalidTokenError()
	}
	if !sharedomain.IsEntityID(claims.Subject) {
		return "", sharedomain.NewInvalidTokenError()
	}
	return strings.ToLower(claims.Subject), nil
}
