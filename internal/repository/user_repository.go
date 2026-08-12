package repository

import (
	"context"
	"fmt"
	"goshop/internal/db/sqlc"
)

type SqlUserRepository struct {
	db sqlc.Querier
}

func NewSqlUserRepository(db sqlc.Querier) UserRepository {
	return &SqlUserRepository{
		db: db,
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
			Search: search,
		})
	case orderBy == "user_id" && sort == "desc":
		users, err = ur.db.ListUserUserIdDesc(ctx, sqlc.ListUserUserIdDescParams{
			Limit:  limit,
			Offset: offset,
			Search: search,
		})
	case orderBy == "user_created_at" && sort == "asc":
		users, err = ur.db.ListUserUserCreatedAtAsc(ctx, sqlc.ListUserUserCreatedAtAscParams{
			Limit:  limit,
			Offset: offset,
			Search: search,
		})
	case orderBy == "user_created_at" && sort == "desc":
		users, err = ur.db.ListUserUserCreatedAtDesc(ctx, sqlc.ListUserUserCreatedAtDescParams{
			Limit:  limit,
			Offset: offset,
			Search: search,
		})
	default:
		return []sqlc.User{}, fmt.Errorf("unsupported sort: order_by=%s sort=%s", orderBy, sort)
	}
	if err != nil {
		return []sqlc.User{}, fmt.Errorf("failed to get users: %w", err)
	}

	return users, nil
}

func (ur *SqlUserRepository) CountUsers(ctx context.Context, search string) (int64, error) {
	total, err := ur.db.CountUsers(ctx, search)
	if err != nil {
		return 0, err
	}

	return total, nil
}

func (ur *SqlUserRepository) Create(ctx context.Context, userParams sqlc.CreateUserParams) (sqlc.User, error) {
	user, err := ur.db.CreateUser(ctx, userParams)
	if err != nil {
		return sqlc.User{}, err
	}
	return user, nil
}

func (ur *SqlUserRepository) FindByUUID(uuid string) {
}

func (ur *SqlUserRepository) FindByEmail(email string) {
}

func (ur *SqlUserRepository) Update(uuid string) {
}

func (ur *SqlUserRepository) Delete(uuid string) {
}
