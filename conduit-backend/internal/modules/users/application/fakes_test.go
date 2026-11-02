package application

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"conduit/backend/internal/modules/users/domain"
)

type memoryUsers struct {
	byID   map[string]domain.User
	order  []string
	nextID int
}

func newMemoryUsers() *memoryUsers {
	return &memoryUsers{byID: map[string]domain.User{}}
}

func (r *memoryUsers) NextID() string {
	r.nextID++
	return fmt.Sprintf("user-%d", r.nextID)
}

func (r *memoryUsers) Add(_ context.Context, user domain.User) error {
	r.byID[user.ID()] = user
	r.order = append(r.order, user.ID())
	return nil
}

func (r *memoryUsers) Update(_ context.Context, user domain.User) (bool, error) {
	if _, ok := r.byID[user.ID()]; !ok {
		return false, nil
	}
	r.byID[user.ID()] = user
	return true, nil
}

func (r *memoryUsers) FindByID(_ context.Context, id string) (domain.User, bool, error) {
	user, ok := r.byID[id]
	return user, ok, nil
}

func (r *memoryUsers) FindByIDs(_ context.Context, ids []string) ([]domain.User, error) {
	found := []domain.User{}
	for _, id := range ids {
		if user, ok := r.byID[id]; ok {
			found = append(found, user)
		}
	}
	return found, nil
}

func (r *memoryUsers) find(match func(domain.User) bool) (domain.User, bool) {
	for _, id := range r.order {
		if user := r.byID[id]; match(user) {
			return user, true
		}
	}
	return domain.User{}, false
}

func (r *memoryUsers) FindByEmail(_ context.Context, email string) (domain.User, bool, error) {
	user, ok := r.find(func(u domain.User) bool { return strings.EqualFold(u.Email().Value(), email) })
	return user, ok, nil
}

func (r *memoryUsers) FindByUsername(_ context.Context, username string) (domain.User, bool, error) {
	user, ok := r.find(func(u domain.User) bool { return strings.EqualFold(u.Username().Value(), username) })
	return user, ok, nil
}

func (r *memoryUsers) ExistsByID(_ context.Context, id string) (bool, error) {
	_, ok := r.byID[id]
	return ok, nil
}

func (r *memoryUsers) FindTakenFields(_ context.Context, query domain.TakenQuery) ([]string, error) {
	taken := []string{}
	for _, id := range r.order {
		user := r.byID[id]
		if id == query.ExcludeID {
			continue
		}
		if query.Email != "" && strings.EqualFold(user.Email().Value(), query.Email) && !slices.Contains(taken, "email") {
			taken = append(taken, "email")
		}
		if query.Username != "" && strings.EqualFold(user.Username().Value(), query.Username) && !slices.Contains(taken, "username") {
			taken = append(taken, "username")
		}
	}
	return taken, nil
}

type memoryFollows struct {
	pairs map[string]bool
}

func newMemoryFollows() *memoryFollows {
	return &memoryFollows{pairs: map[string]bool{}}
}

func (r *memoryFollows) Follow(_ context.Context, followerID, followedID string) error {
	r.pairs[followerID+">"+followedID] = true
	return nil
}

func (r *memoryFollows) Unfollow(_ context.Context, followerID, followedID string) error {
	delete(r.pairs, followerID+">"+followedID)
	return nil
}

func (r *memoryFollows) IsFollowing(_ context.Context, followerID, followedID string) (bool, error) {
	return r.pairs[followerID+">"+followedID], nil
}

func (r *memoryFollows) FindFollowedAmong(_ context.Context, followerID string, candidateIDs []string) (map[string]bool, error) {
	followed := map[string]bool{}
	for _, id := range candidateIDs {
		if r.pairs[followerID+">"+id] {
			followed[id] = true
		}
	}
	return followed, nil
}

func (r *memoryFollows) ListFollowedIDs(_ context.Context, followerID string) ([]string, error) {
	ids := []string{}
	for pair := range r.pairs {
		if follower, followed, ok := strings.Cut(pair, ">"); ok && follower == followerID {
			ids = append(ids, followed)
		}
	}
	slices.Sort(ids)
	return ids, nil
}

type fakeHasher struct{}

func (fakeHasher) Hash(_ context.Context, plain string) (string, error) {
	return "hashed:" + plain, nil
}

func (fakeHasher) Compare(_ context.Context, plain, hash string) bool {
	return hash == "hashed:"+plain
}

type fakeTokens struct{}

func (fakeTokens) Issue(userID string) (string, error) {
	return "token-for-" + userID, nil
}

func (fakeTokens) Verify(token string) (string, error) {
	if id, ok := strings.CutPrefix(token, "token-for-"); ok {
		return id, nil
	}
	return "", fmt.Errorf("invalid token %q", token)
}
