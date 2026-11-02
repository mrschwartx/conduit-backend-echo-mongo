package articles

import (
	"github.com/labstack/echo/v5"

	"conduit/backend/internal/modules/articles/application"
	"conduit/backend/internal/modules/articles/infrastructure"
	"conduit/backend/internal/modules/articles/presentation/rest"
	sharedapp "conduit/backend/internal/shared/application"
	"conduit/backend/internal/shared/infrastructure/postgres"
)

type Dependencies struct {
	DB                   postgres.DB
	Events               sharedapp.EventBus
	Authors              sharedapp.AuthorDirectory
	Authenticate         echo.MiddlewareFunc
	OptionalAuthenticate echo.MiddlewareFunc
}

type UseCases struct {
	CreateArticle     *application.CreateArticle
	GetArticle        *application.GetArticle
	ListArticles      *application.ListArticles
	GetFeed           *application.GetFeed
	UpdateArticle     *application.UpdateArticle
	DeleteArticle     *application.DeleteArticle
	FavoriteArticle   *application.SetFavorite
	UnfavoriteArticle *application.SetFavorite
	ListPopularTags   *application.ListPopularTags
}

type Module struct {
	UseCases UseCases
	Catalog  *application.ArticleCatalog

	handler              *rest.Handler
	authenticate         echo.MiddlewareFunc
	optionalAuthenticate echo.MiddlewareFunc
}

func NewModule(deps Dependencies) *Module {
	repository := infrastructure.NewPostgresRepository(deps.DB)
	slugs := application.NewArticleSlugs(repository)

	useCases := UseCases{
		CreateArticle:     application.NewCreateArticle(repository, slugs, deps.Authors),
		GetArticle:        application.NewGetArticle(repository, deps.Authors),
		ListArticles:      application.NewListArticles(repository, deps.Authors),
		GetFeed:           application.NewGetFeed(repository, deps.Authors),
		UpdateArticle:     application.NewUpdateArticle(repository, slugs, deps.Authors),
		DeleteArticle:     application.NewDeleteArticle(repository, deps.Events),
		FavoriteArticle:   application.NewSetFavorite(repository, deps.Authors, true),
		UnfavoriteArticle: application.NewSetFavorite(repository, deps.Authors, false),
		ListPopularTags:   application.NewListPopularTags(repository),
	}

	return &Module{
		UseCases: useCases,
		Catalog:  application.NewArticleCatalog(repository),
		handler: rest.NewHandler(
			useCases.CreateArticle, useCases.GetArticle, useCases.ListArticles, useCases.GetFeed,
			useCases.UpdateArticle, useCases.DeleteArticle, useCases.FavoriteArticle,
			useCases.UnfavoriteArticle, useCases.ListPopularTags,
		),
		authenticate:         deps.Authenticate,
		optionalAuthenticate: deps.OptionalAuthenticate,
	}
}

func (m *Module) RegisterRoutes(e *echo.Echo) {
	rest.RegisterRoutes(e, m.handler, m.authenticate, m.optionalAuthenticate)
}
