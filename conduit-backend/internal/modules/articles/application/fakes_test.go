package application

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"conduit/backend/internal/modules/articles/domain"
	sharedapp "conduit/backend/internal/shared/application"
	sharedomain "conduit/backend/internal/shared/domain"
)

type memoryArticles struct {
	articles  []domain.Article
	favorites map[string]map[string]bool
	nextID    int
	slugClash int
}

func newMemoryArticles() *memoryArticles {
	return &memoryArticles{favorites: map[string]map[string]bool{}}
}

func (r *memoryArticles) NextID() string {
	r.nextID++
	return fmt.Sprintf("article-%d", r.nextID)
}

func (r *memoryArticles) index(id string) int {
	return slices.IndexFunc(r.articles, func(a domain.Article) bool { return a.ID() == id })
}

func (r *memoryArticles) Add(_ context.Context, article domain.Article) error {
	if r.slugClash > 0 {
		r.slugClash--
		return domain.ErrSlugTaken
	}
	r.articles = append(r.articles, article)
	return nil
}

func (r *memoryArticles) Update(_ context.Context, article domain.Article) (bool, error) {
	i := r.index(article.ID())
	if i < 0 {
		return false, nil
	}
	r.articles[i] = article
	return true, nil
}

func (r *memoryArticles) FindBySlug(_ context.Context, slug string) (domain.Article, bool, error) {
	for _, article := range r.articles {
		if article.Slug() == slug {
			return article, true, nil
		}
	}
	return domain.Article{}, false, nil
}

func (r *memoryArticles) view(article domain.Article, viewerID string) domain.View {
	snapshot := article.Snapshot()
	return domain.View{
		ID: snapshot.ID, AuthorID: snapshot.AuthorID, Slug: snapshot.Slug, Title: snapshot.Title,
		Description: snapshot.Description, Body: snapshot.Body, TagList: snapshot.TagList,
		CreatedAt: snapshot.CreatedAt, UpdatedAt: snapshot.UpdatedAt,
		FavoritesCount: len(r.favorites[snapshot.ID]), Favorited: r.favorites[snapshot.ID][viewerID],
	}
}

func (r *memoryArticles) FindViewBySlug(ctx context.Context, slug, viewerID string) (domain.View, bool, error) {
	article, found, _ := r.FindBySlug(ctx, slug)
	if !found {
		return domain.View{}, false, nil
	}
	return r.view(article, viewerID), true, nil
}

func (r *memoryArticles) FindPage(_ context.Context, query domain.PageQuery) (domain.Page, error) {
	matched := []domain.View{}
	for i := len(r.articles) - 1; i >= 0; i-- {
		article := r.articles[i]
		if query.Tag != nil && !slices.Contains(article.TagList().Values(), *query.Tag) {
			continue
		}
		if query.AuthorID != "" && article.AuthorID() != query.AuthorID {
			continue
		}
		if query.AuthorIDs != nil && !slices.Contains(query.AuthorIDs, article.AuthorID()) {
			continue
		}
		if query.FavoritedByID != "" && !r.favorites[article.ID()][query.FavoritedByID] {
			continue
		}
		matched = append(matched, r.view(article, query.ViewerID))
	}
	start := min(query.Offset, len(matched))
	end := min(start+query.Limit, len(matched))
	return domain.Page{Items: matched[start:end], Total: len(matched)}, nil
}

func (r *memoryArticles) SlugTaken(_ context.Context, slug, excludeID string) (bool, error) {
	for _, article := range r.articles {
		if article.Slug() == slug && article.ID() != excludeID {
			return true, nil
		}
	}
	return false, nil
}

func (r *memoryArticles) DeleteByID(_ context.Context, id string) (bool, error) {
	i := r.index(id)
	if i < 0 {
		return false, nil
	}
	r.articles = slices.Delete(r.articles, i, i+1)
	delete(r.favorites, id)
	return true, nil
}

func (r *memoryArticles) Favorite(_ context.Context, articleID, userID string) error {
	if r.favorites[articleID] == nil {
		r.favorites[articleID] = map[string]bool{}
	}
	r.favorites[articleID][userID] = true
	return nil
}

func (r *memoryArticles) Unfavorite(_ context.Context, articleID, userID string) error {
	delete(r.favorites[articleID], userID)
	return nil
}

func (r *memoryArticles) PopularTags(_ context.Context, limit int) ([]string, error) {
	counts := map[string]int{}
	for _, article := range r.articles {
		for _, tag := range article.TagList().Values() {
			counts[tag]++
		}
	}
	names := make([]string, 0, len(counts))
	for name := range counts {
		names = append(names, name)
	}
	slices.SortFunc(names, func(a, b string) int {
		if counts[a] != counts[b] {
			return counts[b] - counts[a]
		}
		return strings.Compare(a, b)
	})
	if len(names) > limit {
		names = names[:limit]
	}
	return names, nil
}

type fakeAuthors struct {
	usernames map[string]string
	followed  map[string][]string
}

func newFakeAuthors() *fakeAuthors {
	return &fakeAuthors{
		usernames: map[string]string{"jane": "user-1", "john": "user-2"},
		followed:  map[string][]string{},
	}
}

func (a *fakeAuthors) FindProfiles(_ context.Context, userIDs []string, viewerID string) (map[string]sharedapp.AuthorProfile, error) {
	profiles := map[string]sharedapp.AuthorProfile{}
	for _, id := range userIDs {
		for username, userID := range a.usernames {
			if userID == id {
				profiles[id] = sharedapp.AuthorProfile{Username: username, Following: slices.Contains(a.followed[viewerID], id)}
			}
		}
	}
	return profiles, nil
}

func (a *fakeAuthors) FindIDByUsername(_ context.Context, username string) (string, bool, error) {
	id, ok := a.usernames[username]
	return id, ok, nil
}

func (a *fakeAuthors) ListFollowedIDs(_ context.Context, userID string) ([]string, error) {
	return a.followed[userID], nil
}

type recordingBus struct {
	events []sharedomain.Event
}

func (b *recordingBus) Publish(_ context.Context, event sharedomain.Event) {
	b.events = append(b.events, event)
}

func (b *recordingBus) Subscribe(string, sharedapp.EventHandler) {}
