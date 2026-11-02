package domain

import (
	"regexp"

	sharedomain "conduit/backend/internal/shared/domain"
)

var usernamePattern = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

var usernameRule = sharedomain.TextRule{Min: 3, Max: 30, Pattern: usernamePattern}

type Username struct {
	value string
}

func NewUsername(raw sharedomain.Raw) (Username, error) {
	value, err := sharedomain.ValidateText("username", raw, usernameRule)
	if err != nil {
		return Username{}, err
	}
	return Username{value: value}, nil
}

func RestoreUsername(value string) Username {
	return Username{value: value}
}

func (u Username) Value() string {
	return u.value
}

func (u Username) Equals(other Username) bool {
	return u.value == other.value
}
