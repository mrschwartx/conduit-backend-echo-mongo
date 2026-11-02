package application

import (
	"conduit/backend/internal/modules/articles/domain"
	sharedomain "conduit/backend/internal/shared/domain"
)

type ArticleInput struct {
	Title       sharedomain.Raw
	Description sharedomain.Raw
	Body        sharedomain.Raw
	TagList     sharedomain.Raw
}

type parsedFields struct {
	title       *domain.ArticleTitle
	description *domain.ArticleDescription
	body        *domain.ArticleBody
	tagList     *domain.TagList
}

func (input ArticleInput) hasAny() bool {
	return input.Title.IsSet() || input.Description.IsSet() || input.Body.IsSet() || input.TagList.IsSet()
}

func parseArticleFields(input ArticleInput, all bool) (parsedFields, error) {
	errs := sharedomain.NewErrorCollector()
	var fields parsedFields

	if all || input.Title.IsSet() {
		value, err := domain.NewArticleTitle(input.Title)
		if captured := errs.Capture(err); captured != nil {
			return fields, captured
		}
		if err == nil {
			fields.title = &value
		}
	}
	if all || input.Description.IsSet() {
		value, err := domain.NewArticleDescription(input.Description)
		if captured := errs.Capture(err); captured != nil {
			return fields, captured
		}
		if err == nil {
			fields.description = &value
		}
	}
	if all || input.Body.IsSet() {
		value, err := domain.NewArticleBody(input.Body)
		if captured := errs.Capture(err); captured != nil {
			return fields, captured
		}
		if err == nil {
			fields.body = &value
		}
	}
	if all || input.TagList.IsSet() {
		value, err := domain.NewTagList(input.TagList)
		if captured := errs.Capture(err); captured != nil {
			return fields, captured
		}
		if err == nil {
			fields.tagList = &value
		}
	}
	return fields, errs.Err()
}
