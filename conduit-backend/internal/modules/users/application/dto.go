package application

import (
	"conduit/backend/internal/modules/users/domain"
	sharedapp "conduit/backend/internal/shared/application"
)

type UserDTO struct {
	Email    string  `json:"email"`
	Token    string  `json:"token"`
	Username string  `json:"username"`
	Bio      *string `json:"bio"`
	Image    *string `json:"image"`
}

type ProfileDTO = sharedapp.AuthorProfile

func ToUserDTO(user domain.User, token string) UserDTO {
	return UserDTO{
		Email:    user.Email().Value(),
		Token:    token,
		Username: user.Username().Value(),
		Bio:      user.Bio().Value(),
		Image:    user.Image().Value(),
	}
}

func ToProfileDTO(user domain.User, following bool) ProfileDTO {
	return ProfileDTO{
		Username:  user.Username().Value(),
		Bio:       user.Bio().Value(),
		Image:     user.Image().Value(),
		Following: following,
	}
}
