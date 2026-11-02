package domain

import (
	"regexp"
	"strings"

	sharedomain "conduit/backend/internal/shared/domain"
)

var emailPattern = regexp.MustCompile(`^[^\s@]+@[^\s@]+\.[^\s@]+$`)

var emailRule = sharedomain.TextRule{Min: 1, Max: 254, Pattern: emailPattern}

type Email struct {
	value string
}

func NewEmail(raw sharedomain.Raw) (Email, error) {
	value, err := sharedomain.ValidateText("email", raw, emailRule)
	if err != nil {
		return Email{}, err
	}
	return Email{value: strings.ToLower(value)}, nil
}

func RestoreEmail(value string) Email {
	return Email{value: value}
}

func (e Email) Value() string {
	return e.value
}

func (e Email) Equals(other Email) bool {
	return e.value == other.value
}
