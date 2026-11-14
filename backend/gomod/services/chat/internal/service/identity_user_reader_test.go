package service_test

import (
	"context"
	"testing"

	chatservice "goshop/services/chat/internal/service"
	identitycontract "goshop/services/identity/contract"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

// fakeIdentityClient stubs only the lookup the reader under test performs.
type fakeIdentityClient struct {
	identitycontract.IdentityClient
	byID map[int32]*identitycontract.InternalUserDTO
}

func (f *fakeIdentityClient) GetUserByID(_ context.Context, id int32) (*identitycontract.InternalUserDTO, error) {
	return f.byID[id], nil
}

// The avatar feature writes user_avatar_url. The legacy user_avatar column is
// empty for everyone who set an avatar since, so preferring it made every chat
// conversation render the initial-letter fallback.
func TestIdentityUserReader_PrefersAvatarURLCoverLegacyColumn(t *testing.T) {
	url := "https://cdn.example.com/goshop-media/avatars/a.png"
	reader := chatservice.NewIdentityUserReader(&fakeIdentityClient{byID: map[int32]*identitycontract.InternalUserDTO{
		7: {UserID: 7, UserUUID: uuid.New(), UserAvatar: "", UserAvatarUrl: &url},
	}})

	user, err := reader.GetByID(context.Background(), 7)
	assert.NoError(t, err)
	assert.Equal(t, url, user.UserAvatar)
}

func TestIdentityUserReader_FallsBackToLegacyAvatar(t *testing.T) {
	reader := chatservice.NewIdentityUserReader(&fakeIdentityClient{byID: map[int32]*identitycontract.InternalUserDTO{
		8: {UserID: 8, UserUUID: uuid.New(), UserAvatar: "https://cdn/legacy.png"},
	}})

	user, err := reader.GetByID(context.Background(), 8)
	assert.NoError(t, err)
	assert.Equal(t, "https://cdn/legacy.png", user.UserAvatar)
}
