package service

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"

	"goshop/services/identity/internal/db/sqlc"
	dto "goshop/services/identity/internal/dto"
	"goshop/services/identity/internal/repository"
)

// avatarRepo captures the params UpdateAvatar forwarded, so the prefix rule can
// be asserted without a database.
type avatarRepo struct {
	repository.UserRepository
	got sqlc.UpdateUserAvatarParams
}

func (r *avatarRepo) UpdateAvatar(_ context.Context, input sqlc.UpdateUserAvatarParams) (sqlc.User, error) {
	r.got = input
	u := sqlc.User{UserUuid: input.UserUuid, UserFullname: "Test User", UserEmail: "t@example.com"}
	u.UserAvatarUrl = input.AvatarUrl
	return u, nil
}

func newAvatarService(t *testing.T) (*userService, *avatarRepo) {
	t.Helper()
	repo := &avatarRepo{}
	return &userService{repo: repo}, repo
}

// The service refuses any key outside avatars/, which is the boundary that keeps
// an avatar update from claiming a catalog or review object.
func TestUpdateAvatarRejectsKeysOutsideAvatars(t *testing.T) {
	for _, key := range []string{
		"products/x.png",
		"/products/x.png",
		"avatars/../../products/x.png",
		"reviews/x.jpg",
	} {
		svc, repo := newAvatarService(t)
		if _, err := svc.UpdateAvatar(context.Background(), uuid.New(), key); err == nil {
			t.Fatalf("key %q was accepted", key)
		}
		if repo.got.AvatarUrl != nil {
			t.Fatalf("key %q reached the repository", key)
		}
	}
}

func TestUpdateAvatarStoresAvatarKey(t *testing.T) {
	svc, repo := newAvatarService(t)
	if _, err := svc.UpdateAvatar(context.Background(), uuid.New(), "avatars/abc.png"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo.got.AvatarUrl == nil || !strings.HasSuffix(*repo.got.AvatarUrl, "avatars/abc.png") {
		t.Fatalf("stored avatar = %v, want it to end with avatars/abc.png", repo.got.AvatarUrl)
	}
}

// An empty key is the documented "remove my avatar" request, not an error.
func TestUpdateAvatarClearsOnEmptyKey(t *testing.T) {
	svc, repo := newAvatarService(t)
	if _, err := svc.UpdateAvatar(context.Background(), uuid.New(), ""); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if repo.got.AvatarUrl != nil {
		t.Fatalf("stored avatar = %v, want nil", repo.got.AvatarUrl)
	}
}

func TestAvatarPathIsEdgeRelative(t *testing.T) {
	t.Setenv("S3_PUBLIC_BASE_URL", "https://cdn.example.com/goshop-media")

	abs := "https://cdn.example.com/goshop-media/avatars/a.png"
	if got := dto.AvatarPath(&abs); got != "/goshop-media/avatars/a.png" {
		t.Fatalf("AvatarPath(%q) = %q", abs, got)
	}

	// External provider avatars (Google) must survive untouched.
	ext := "https://lh3.googleusercontent.com/a/photo.jpg"
	if got := dto.AvatarPath(&ext); got != ext {
		t.Fatalf("external avatar was rewritten to %q", got)
	}

	if got := dto.AvatarPath(nil); got != "" {
		t.Fatalf("nil avatar = %q, want empty", got)
	}
}
