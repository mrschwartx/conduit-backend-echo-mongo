package domain

import (
	"net/url"

	sharedomain "conduit/backend/internal/shared/domain"
)

const imageMaxLength = 2048

type ProfileImage struct {
	value string
}

func NewProfileImage(raw sharedomain.Raw) (*ProfileImage, error) {
	if raw.Value() == nil {
		return nil, nil
	}
	text, ok := raw.Text()
	if !ok {
		return nil, sharedomain.NewFieldError("image", sharedomain.Invalid)
	}
	value := sharedomain.TrimText(text)
	if value == "" {
		return nil, nil
	}
	if sharedomain.CountCharacters(value) > imageMaxLength {
		return nil, sharedomain.NewFieldError("image", sharedomain.TooLong(imageMaxLength))
	}
	if !isHTTPSURL(value) {
		return nil, sharedomain.NewFieldError("image", "must be a valid https URL")
	}
	return &ProfileImage{value: value}, nil
}

func RestoreProfileImage(value *string) *ProfileImage {
	if value == nil {
		return nil
	}
	return &ProfileImage{value: *value}
}

func (i *ProfileImage) Value() *string {
	if i == nil {
		return nil
	}
	value := i.value
	return &value
}

func isHTTPSURL(value string) bool {
	parsed, err := url.Parse(value)
	return err == nil && parsed.Scheme == "https" && parsed.Hostname() != ""
}
