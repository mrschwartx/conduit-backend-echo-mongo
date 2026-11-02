package domain

import sharedomain "conduit/backend/internal/shared/domain"

var (
	titleRule       = sharedomain.TextRule{Min: 1, Max: 100}
	descriptionRule = sharedomain.TextRule{Min: 1, Max: 200}
	bodyRule        = sharedomain.TextRule{Min: 1, Max: 20000}
)

type ArticleTitle struct{ value string }

func NewArticleTitle(raw sharedomain.Raw) (ArticleTitle, error) {
	value, err := sharedomain.ValidateText("title", raw, titleRule)
	return ArticleTitle{value: value}, err
}

func RestoreArticleTitle(value string) ArticleTitle { return ArticleTitle{value: value} }

func (v ArticleTitle) Value() string { return v.value }

type ArticleDescription struct{ value string }

func NewArticleDescription(raw sharedomain.Raw) (ArticleDescription, error) {
	value, err := sharedomain.ValidateText("description", raw, descriptionRule)
	return ArticleDescription{value: value}, err
}

func RestoreArticleDescription(value string) ArticleDescription {
	return ArticleDescription{value: value}
}

func (v ArticleDescription) Value() string { return v.value }

type ArticleBody struct{ value string }

func NewArticleBody(raw sharedomain.Raw) (ArticleBody, error) {
	value, err := sharedomain.ValidateText("body", raw, bodyRule)
	return ArticleBody{value: value}, err
}

func RestoreArticleBody(value string) ArticleBody { return ArticleBody{value: value} }

func (v ArticleBody) Value() string { return v.value }
