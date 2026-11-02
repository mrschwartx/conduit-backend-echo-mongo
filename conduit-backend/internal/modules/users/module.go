package users

import (
	"github.com/labstack/echo/v5"

	"conduit/backend/internal/config"
	"conduit/backend/internal/modules/users/application"
	"conduit/backend/internal/modules/users/infrastructure"
	"conduit/backend/internal/modules/users/presentation/rest"
	"conduit/backend/internal/shared/infrastructure/postgres"
)

type Dependencies struct {
	DB     postgres.Querier
	Config config.Config
}

type UseCases struct {
	RegisterUser      *application.RegisterUser
	AuthenticateUser  *application.AuthenticateUser
	VerifyAccessToken *application.VerifyAccessToken
	GetCurrentUser    *application.GetCurrentUser
	UpdateCurrentUser *application.UpdateCurrentUser
	GetProfile        *application.GetProfile
	FollowUser        *application.FollowUser
	UnfollowUser      *application.UnfollowUser
}

type Module struct {
	UseCases             UseCases
	AuthorDirectory      *application.AuthorDirectory
	Authenticate         echo.MiddlewareFunc
	OptionalAuthenticate echo.MiddlewareFunc

	usersHandler    *rest.UsersHandler
	profilesHandler *rest.ProfilesHandler
}

func NewModule(deps Dependencies) *Module {
	cfg := deps.Config
	users := infrastructure.NewPostgresRepository(deps.DB)
	follows := infrastructure.NewPostgresFollowRepository(deps.DB)
	hasher := infrastructure.NewBcryptHasher(cfg.Auth.BcryptRounds)
	tokens := infrastructure.NewJWTTokenService(cfg.Auth.Secret, cfg.Auth.TokenTTL)

	useCases := UseCases{
		RegisterUser:      application.NewRegisterUser(users, hasher, tokens),
		AuthenticateUser:  application.NewAuthenticateUser(users, hasher, tokens),
		VerifyAccessToken: application.NewVerifyAccessToken(tokens, users),
		GetCurrentUser:    application.NewGetCurrentUser(users, tokens),
		UpdateCurrentUser: application.NewUpdateCurrentUser(users, hasher, tokens),
		GetProfile:        application.NewGetProfile(users, follows),
		FollowUser:        application.NewFollowUser(users, follows),
		UnfollowUser:      application.NewUnfollowUser(users, follows),
	}

	return &Module{
		UseCases:             useCases,
		AuthorDirectory:      application.NewAuthorDirectory(users, follows),
		Authenticate:         rest.Authenticate(useCases.VerifyAccessToken, false),
		OptionalAuthenticate: rest.Authenticate(useCases.VerifyAccessToken, true),
		usersHandler: rest.NewUsersHandler(
			useCases.RegisterUser, useCases.AuthenticateUser, useCases.GetCurrentUser, useCases.UpdateCurrentUser,
		),
		profilesHandler: rest.NewProfilesHandler(useCases.GetProfile, useCases.FollowUser, useCases.UnfollowUser),
	}
}

func (m *Module) RegisterRoutes(e *echo.Echo) {
	rest.RegisterRoutes(e, m.usersHandler, m.profilesHandler, m.Authenticate, m.OptionalAuthenticate)
}
