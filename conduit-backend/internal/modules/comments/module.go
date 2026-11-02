package comments

import (
	"context"

	"github.com/labstack/echo/v5"

	"conduit/backend/internal/modules/comments/application"
	"conduit/backend/internal/modules/comments/infrastructure"
	"conduit/backend/internal/modules/comments/presentation/rest"
	sharedapp "conduit/backend/internal/shared/application"
	sharedomain "conduit/backend/internal/shared/domain"
	"conduit/backend/internal/shared/infrastructure/postgres"
)

type Dependencies struct {
	DB                   postgres.Querier
	Events               sharedapp.EventBus
	Authors              sharedapp.AuthorDirectory
	Articles             application.ArticleCatalog
	Authenticate         echo.MiddlewareFunc
	OptionalAuthenticate echo.MiddlewareFunc
}

type UseCases struct {
	CreateComment           *application.CreateComment
	ListComments            *application.ListComments
	DeleteComment           *application.DeleteComment
	DeleteCommentsByArticle *application.DeleteCommentsByArticle
}

type Module struct {
	UseCases UseCases

	handler              *rest.Handler
	authenticate         echo.MiddlewareFunc
	optionalAuthenticate echo.MiddlewareFunc
}

func NewModule(deps Dependencies) *Module {
	repository := infrastructure.NewPostgresRepository(deps.DB)

	useCases := UseCases{
		CreateComment:           application.NewCreateComment(repository, deps.Articles, deps.Authors),
		ListComments:            application.NewListComments(repository, deps.Articles, deps.Authors),
		DeleteComment:           application.NewDeleteComment(repository, deps.Articles),
		DeleteCommentsByArticle: application.NewDeleteCommentsByArticle(repository),
	}

	deps.Events.Subscribe(sharedomain.ArticleDeletedEvent, func(ctx context.Context, event sharedomain.Event) error {
		payload, ok := event.Payload.(sharedomain.ArticleDeleted)
		if !ok {
			return nil
		}
		_, err := useCases.DeleteCommentsByArticle.Execute(ctx, payload.ArticleID)
		return err
	})

	return &Module{
		UseCases:             useCases,
		handler:              rest.NewHandler(useCases.CreateComment, useCases.ListComments, useCases.DeleteComment),
		authenticate:         deps.Authenticate,
		optionalAuthenticate: deps.OptionalAuthenticate,
	}
}

func (m *Module) RegisterRoutes(e *echo.Echo) {
	rest.RegisterRoutes(e, m.handler, m.authenticate, m.optionalAuthenticate)
}
