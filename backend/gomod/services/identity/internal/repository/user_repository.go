package repository

import (
	"context"
	"fmt"
	"goshop/services/identity/internal/db/sqlc"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

type SqlUserRepository struct {
	db   sqlc.Querier
	pool *pgxpool.Pool
}

func NewSqlUserRepository(querier sqlc.Querier, pool ...*pgxpool.Pool) UserRepository {
	var p *pgxpool.Pool
	if len(pool) > 0 && pool[0] != nil {
		p = pool[0]
	}
	return &SqlUserRepository{
		db:   querier,
		pool: p,
	}
}

func (ur *SqlUserRepository) GetAll(ctx context.Context, search, orderBy, sort string, limit, offset int32) ([]sqlc.User, error) {
	var (
		users []sqlc.User
		err   error
	)

	switch {
	case orderBy == "user_id" && sort == "asc":
		users, err = ur.db.ListUserUserIdAsc(ctx, sqlc.ListUserUserIdAscParams{
			Limit:  limit,
			Offset: offset,
			Search: &search,
		})
	case orderBy == "user_id" && sort == "desc":
		users, err = ur.db.ListUserUserIdDesc(ctx, sqlc.ListUserUserIdDescParams{
			Limit:  limit,
			Offset: offset,
			Search: &search,
		})
	case orderBy == "user_created_at" && sort == "asc":
		users, err = ur.db.ListUserUserCreatedAtAsc(ctx, sqlc.ListUserUserCreatedAtAscParams{
			Limit:  limit,
			Offset: offset,
			Search: &search,
		})
	case orderBy == "user_created_at" && sort == "desc":
		users, err = ur.db.ListUserUserCreatedAtDesc(ctx, sqlc.ListUserUserCreatedAtDescParams{
			Limit:  limit,
			Offset: offset,
			Search: &search,
		})
	default:
		return []sqlc.User{}, fmt.Errorf("unsupported sort: order_by=%s sort=%s", orderBy, sort)
	}
	if err != nil {
		return []sqlc.User{}, fmt.Errorf("failed to get users: %w", err)
	}

	return users, nil
}

func (ur *SqlUserRepository) GetAllV2(ctx context.Context, search, orderBy, sort string, limit, offset int32, deleted bool) ([]sqlc.User, error) {
	query := `SELECT user_id, user_uuid, user_fullname, user_email, user_password, user_age, user_status, user_level, user_deleted_at, user_created_at, user_updated_at, user_avatar
			FROM users
			WHERE (
				$1::TEXT IS NULL
				OR $1::TEXT = ''
				OR unaccent(user_email) ILIKE '%' || unaccent($1) || '%'
				OR unaccent(user_fullname) ILIKE '%' || unaccent($1) || '%'
			)`

	if deleted {
		query += " AND user_deleted_at IS NOT NULL"
	} else {
		query += " AND user_deleted_at IS NULL"
	}

	order := "ASC"
	if sort == "desc" {
		order = "DESC"
	}

	switch orderBy {
	case "user_id", "user_created_at":
		query += fmt.Sprintf(" ORDER BY %s %s", orderBy, order)
	default:
		query += " ORDER BY user_id ASC"
	}

	query += " LIMIT $2 OFFSET $3"

	pool := ur.pool
	if pool == nil {
		return nil, fmt.Errorf("database pool is not initialized")
	}

	rows, err := pool.Query(ctx, query, search, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []sqlc.User
	for rows.Next() {
		var i sqlc.User
		if err := rows.Scan(
			&i.UserID,
			&i.UserUuid,
			&i.UserFullname,
			&i.UserEmail,
			&i.UserPassword,
			&i.UserAge,
			&i.UserStatus,
			&i.UserLevel,
			&i.UserDeletedAt,
			&i.UserCreatedAt,
			&i.UserUpdatedAt,
			&i.UserAvatar,
		); err != nil {
			return nil, err
		}
		users = append(users, i)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return users, nil
}

func (ur *SqlUserRepository) CountUsers(ctx context.Context, search string, deleted bool) (int64, error) {
	total, err := ur.db.CountUsers(ctx, sqlc.CountUsersParams{
		Search:  &search,
		Deleted: &deleted,
	})
	if err != nil {
		return 0, err
	}
	return total, nil
}

// CountCustomersCreatedBetween counts registered customers (user_level 3,
// not soft-deleted) whose user_created_at falls inside [fromTime, toTime].
func (ur *SqlUserRepository) CountCustomersCreatedBetween(ctx context.Context, fromTime, toTime time.Time) (int64, error) {
	return ur.db.CountCustomersCreatedBetween(ctx, sqlc.CountCustomersCreatedBetweenParams{
		FromTime: fromTime,
		ToTime:   toTime,
	})
}

func (ur *SqlUserRepository) Create(ctx context.Context, userParams sqlc.CreateUserParams) (sqlc.User, error) {
	return ur.db.CreateUser(ctx, userParams)
}

func (ur *SqlUserRepository) GetByID(ctx context.Context, id int32) (sqlc.User, error) {
	return ur.db.GetUserByID(ctx, id)
}

func (ur *SqlUserRepository) GetByUUID(ctx context.Context, uuid uuid.UUID) (sqlc.User, error) {
	return ur.db.GetUserByUUID(ctx, uuid)
}

func (ur *SqlUserRepository) GetByEmail(ctx context.Context, email string) (sqlc.User, error) {
	return ur.db.GetUserByEmail(ctx, email)
}

func (ur *SqlUserRepository) GetActiveAdminUserIDs(ctx context.Context) ([]int32, error) {
	return ur.db.GetActiveAdminUserIDs(ctx)
}

func (ur *SqlUserRepository) Update(ctx context.Context, input sqlc.UpdateUserParams) (sqlc.User, error) {
	return ur.db.UpdateUser(ctx, input)
}

func (ur *SqlUserRepository) UpdateAvatar(ctx context.Context, input sqlc.UpdateUserAvatarParams) (sqlc.User, error) {
	return ur.db.UpdateUserAvatar(ctx, input)
}

func (ur *SqlUserRepository) SoftDelete(ctx context.Context, uuid uuid.UUID) (sqlc.User, error) {
	return ur.db.SoftDeleteUser(ctx, uuid)
}

func (ur *SqlUserRepository) Restore(ctx context.Context, uuid uuid.UUID) (sqlc.User, error) {
	return ur.db.RestoreUser(ctx, uuid)
}

func (ur *SqlUserRepository) Trash(ctx context.Context, uuid uuid.UUID) (sqlc.User, error) {
	return ur.db.TrashUser(ctx, uuid)
}

func (ur *SqlUserRepository) UpdatePassword(ctx context.Context, input sqlc.UpdatePasswordParams) (sqlc.User, error) {
	return ur.db.UpdatePassword(ctx, input)
}

func (ur *SqlUserRepository) GetByOAuth(ctx context.Context, provider, oauthID string) (sqlc.User, error) {
	return ur.db.GetUserByOAuth(ctx, sqlc.GetUserByOAuthParams{
		Provider: &provider,
		OauthID:  &oauthID,
	})
}

func (ur *SqlUserRepository) CreateOAuthUser(ctx context.Context, params sqlc.CreateOAuthUserParams) (sqlc.User, error) {
	return ur.db.CreateOAuthUser(ctx, params)
}

func (ur *SqlUserRepository) GetByIDs(ctx context.Context, ids []int32) ([]sqlc.GetUsersByIDsRow, error) {
	if len(ids) == 0 {
		return []sqlc.GetUsersByIDsRow{}, nil
	}
	return ur.db.GetUsersByIDs(ctx, ids)
}
