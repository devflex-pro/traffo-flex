package auth

import (
	"context"
	"slices"
	"sync"
)

type MemoryRepository struct {
	mu      sync.RWMutex
	users   map[string]User
	byEmail map[string]string
}

func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{
		users:   map[string]User{},
		byEmail: map[string]string{},
	}
}

func (r *MemoryRepository) ListUsers(ctx context.Context) (
	[]User,
	error,
) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	users := make(
		[]User,
		0,
		len(r.users),
	)
	for _, user := range r.users {
		users = append(
			users,
			user,
		)
	}
	slices.SortFunc(
		users,
		func(
			a,
			b User,
		) int {
			if a.CreatedAt.Before(b.CreatedAt) {
				return -1
			}
			if a.CreatedAt.After(b.CreatedAt) {
				return 1
			}
			return 0
		},
	)
	return users, nil
}

func (r *MemoryRepository) GetUser(
	ctx context.Context,
	id string,
) (
	User,
	error,
) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	user, ok := r.users[id]
	if !ok {
		return User{}, ErrNotFound
	}
	return user, nil
}

func (r *MemoryRepository) GetUserByEmail(
	ctx context.Context,
	email string,
) (
	User,
	error,
) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	id, ok := r.byEmail[normalizeEmail(email)]
	if !ok {
		return User{}, ErrNotFound
	}
	return r.users[id], nil
}

func (r *MemoryRepository) SaveUser(
	ctx context.Context,
	user User,
) (
	User,
	error,
) {
	r.mu.Lock()
	defer r.mu.Unlock()

	user.Email = normalizeEmail(user.Email)
	r.users[user.ID] = user
	r.byEmail[user.Email] = user.ID
	return user, nil
}
