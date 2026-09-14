package v1service_test

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"goshop/internal/db/sqlc"
	v1dto "goshop/internal/dto/v1"
	v1service "goshop/internal/service/v1"
	"goshop/internal/testutils"
	"goshop/internal/utils"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReviewService_CreateReview(t *testing.T) {
	ctx := context.Background()
	userUUID := uuid.New()
	productUUID := uuid.New()
	orderUUID := uuid.New()

	t.Run("Case 1: User Not Found -> Returns 404", func(t *testing.T) {
		mockUserRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return sqlc.User{}, pgx.ErrNoRows
			},
		}

		svc := v1service.NewReviewService(nil, mockUserRepo, nil, nil, nil, nil)
		_, err := svc.CreateReview(ctx, userUUID, v1dto.CreateReviewInput{
			ProductUUID: productUUID.String(),
			OrderUUID:   orderUUID.String(),
			Rating:      5,
		}, nil, nil)

		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeNotFound)
	})

	t.Run("Case 2: Invalid Product UUID -> Returns 400", func(t *testing.T) {
		mockUserRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return sqlc.User{UserID: 1, UserUuid: userUUID}, nil
			},
		}

		svc := v1service.NewReviewService(nil, mockUserRepo, nil, nil, nil, nil)
		_, err := svc.CreateReview(ctx, userUUID, v1dto.CreateReviewInput{
			ProductUUID: "invalid-uuid",
			OrderUUID:   orderUUID.String(),
			Rating:      5,
		}, nil, nil)

		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeBadRequest)
	})

	t.Run("Case 3: Product Not Found -> Returns 404", func(t *testing.T) {
		mockUserRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return sqlc.User{UserID: 1, UserUuid: userUUID}, nil
			},
		}
		mockProductRepo := &testutils.MockProductRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.GetProductByUUIDRow, error) {
				return sqlc.GetProductByUUIDRow{}, sql.ErrNoRows
			},
		}

		svc := v1service.NewReviewService(nil, mockUserRepo, mockProductRepo, nil, nil, nil)
		_, err := svc.CreateReview(ctx, userUUID, v1dto.CreateReviewInput{
			ProductUUID: productUUID.String(),
			OrderUUID:   orderUUID.String(),
			Rating:      5,
		}, nil, nil)

		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeNotFound)
	})

	t.Run("Case 4: User Has Not Purchased Product -> Returns 400", func(t *testing.T) {
		mockUserRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return sqlc.User{UserID: 1, UserUuid: userUUID}, nil
			},
		}
		mockProductRepo := &testutils.MockProductRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.GetProductByUUIDRow, error) {
				return sqlc.GetProductByUUIDRow{ProductID: 10, ProductUuid: productUUID}, nil
			},
		}
		mockOrderRepo := &testutils.MockOrderRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.GetOrderByUUIDRow, error) {
				return sqlc.GetOrderByUUIDRow{OrderID: 100, OrderUuid: orderUUID}, nil
			},
		}
		mockReviewRepo := &testutils.MockReviewRepository{
			CheckPurchasedFunc: func(ctx context.Context, orderID, userID, productID int32) (bool, error) {
				return false, nil // Not purchased
			},
		}

		svc := v1service.NewReviewService(mockReviewRepo, mockUserRepo, mockProductRepo, mockOrderRepo, nil, nil)
		_, err := svc.CreateReview(ctx, userUUID, v1dto.CreateReviewInput{
			ProductUUID: productUUID.String(),
			OrderUUID:   orderUUID.String(),
			Rating:      5,
		}, nil, nil)

		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeBadRequest)
		assert.Contains(t, err.Error(), "not purchased")
	})

	t.Run("Case 5: Review Already Exists -> Returns 400", func(t *testing.T) {
		mockUserRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return sqlc.User{UserID: 1, UserUuid: userUUID}, nil
			},
		}
		mockProductRepo := &testutils.MockProductRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.GetProductByUUIDRow, error) {
				return sqlc.GetProductByUUIDRow{ProductID: 10, ProductUuid: productUUID}, nil
			},
		}
		mockOrderRepo := &testutils.MockOrderRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.GetOrderByUUIDRow, error) {
				return sqlc.GetOrderByUUIDRow{OrderID: 100, OrderUuid: orderUUID}, nil
			},
		}
		mockReviewRepo := &testutils.MockReviewRepository{
			CheckPurchasedFunc: func(ctx context.Context, orderID, userID, productID int32) (bool, error) {
				return true, nil
			},
			CheckExistsFunc: func(ctx context.Context, userID, productID, orderID int32) (bool, error) {
				return true, nil // Already reviewed
			},
		}

		svc := v1service.NewReviewService(mockReviewRepo, mockUserRepo, mockProductRepo, mockOrderRepo, nil, nil)
		_, err := svc.CreateReview(ctx, userUUID, v1dto.CreateReviewInput{
			ProductUUID: productUUID.String(),
			OrderUUID:   orderUUID.String(),
			Rating:      5,
		}, nil, nil)

		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeBadRequest)
		assert.Contains(t, err.Error(), "already exists")
	})
}

func TestReviewService_GetProductReviews(t *testing.T) {
	ctx := context.Background()
	productUUID := uuid.New()
	reviewUUID := uuid.New()

	t.Run("Case 1: Product Not Found -> Returns 404", func(t *testing.T) {
		mockProductRepo := &testutils.MockProductRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.GetProductByUUIDRow, error) {
				return sqlc.GetProductByUUIDRow{}, pgx.ErrNoRows
			},
		}

		svc := v1service.NewReviewService(nil, nil, mockProductRepo, nil, nil, nil)
		_, _, err := svc.GetProductReviews(ctx, productUUID, v1dto.GetReviewsQuery{})

		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeNotFound)
	})

	t.Run("Case 2: Happy Path -> Returns Reviews List and Total", func(t *testing.T) {
		mockProductRepo := &testutils.MockProductRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.GetProductByUUIDRow, error) {
				return sqlc.GetProductByUUIDRow{
					ProductID:          10,
					ProductUuid:        productUUID,
					ProductReviewCount: 1,
				}, nil
			},
		}
		mockReviewRepo := &testutils.MockReviewRepository{
			GetByProductFunc: func(ctx context.Context, productID int32, rating *int32, hasMedia *bool, orderBy, sort string, limit, offset int32) ([]v1dto.ReviewRow, error) {
				return []v1dto.ReviewRow{
					{
						ReviewID:           1,
						ReviewUUID:         reviewUUID,
						ReviewRating:       5,
						ReviewTitle:        utils.StringToPointer("Great product!"),
						ReviewComment:      utils.StringToPointer("Loved the quality."),
						ReviewerName:       "Jane Doe",
						ReviewHelpfulCount: 3,
						ReviewCreatedAt:    time.Now(),
						ReviewUpdatedAt:    time.Now(),
					},
				}, nil
			},
		}

		svc := v1service.NewReviewService(mockReviewRepo, nil, mockProductRepo, nil, nil, nil)
		reviews, total, err := svc.GetProductReviews(ctx, productUUID, v1dto.GetReviewsQuery{})

		require.NoError(t, err)
		assert.Equal(t, int32(1), total)
		assert.Len(t, reviews, 1)
		assert.Equal(t, reviewUUID.String(), reviews[0].UUID)
		assert.Equal(t, int32(5), reviews[0].Rating)
	})
}

func TestReviewService_GetProductRatingSummary(t *testing.T) {
	ctx := context.Background()
	productUUID := uuid.New()

	t.Run("Case 1: Product Not Found -> Returns 404", func(t *testing.T) {
		mockProductRepo := &testutils.MockProductRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.GetProductByUUIDRow, error) {
				return sqlc.GetProductByUUIDRow{}, sql.ErrNoRows
			},
		}

		svc := v1service.NewReviewService(nil, nil, mockProductRepo, nil, nil, nil)
		_, err := svc.GetProductRatingSummary(ctx, productUUID)

		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeNotFound)
	})

	t.Run("Case 2: Happy Path -> Returns Rating Summary", func(t *testing.T) {
		mockProductRepo := &testutils.MockProductRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.GetProductByUUIDRow, error) {
				return sqlc.GetProductByUUIDRow{ProductID: 10, ProductUuid: productUUID}, nil
			},
		}
		mockReviewRepo := &testutils.MockReviewRepository{
			GetRatingSummaryFunc: func(ctx context.Context, productID int32) (sqlc.GetProductRatingSummaryRow, error) {
				return sqlc.GetProductRatingSummaryRow{
					TotalReviews:   10,
					AvgRating:      4.8,
					WithMediaCount: 4,
					FiveStar:       8,
					FourStar:       2,
				}, nil
			},
		}

		svc := v1service.NewReviewService(mockReviewRepo, nil, mockProductRepo, nil, nil, nil)
		summary, err := svc.GetProductRatingSummary(ctx, productUUID)

		require.NoError(t, err)
		assert.Equal(t, int64(10), summary.TotalReviews)
		assert.InDelta(t, 4.8, summary.AvgRating, 0.001)
		assert.Equal(t, int64(8), summary.FiveStar)
	})
}

func TestReviewService_GetMyReviews(t *testing.T) {
	ctx := context.Background()
	userUUID := uuid.New()
	reviewUUID := uuid.New()

	t.Run("Case 1: User Not Found -> Returns 404", func(t *testing.T) {
		mockUserRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return sqlc.User{}, pgx.ErrNoRows
			},
		}

		svc := v1service.NewReviewService(nil, mockUserRepo, nil, nil, nil, nil)
		_, _, err := svc.GetMyReviews(ctx, userUUID, v1dto.GetMyReviewsQuery{})

		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeNotFound)
	})

	t.Run("Case 2: Zero Reviews -> Returns Empty Slice", func(t *testing.T) {
		mockUserRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return sqlc.User{UserID: 1, UserUuid: userUUID}, nil
			},
		}
		mockReviewRepo := &testutils.MockReviewRepository{
			CountByUserIDFunc: func(ctx context.Context, userID int32) (int64, error) {
				return 0, nil
			},
		}

		svc := v1service.NewReviewService(mockReviewRepo, mockUserRepo, nil, nil, nil, nil)
		reviews, total, err := svc.GetMyReviews(ctx, userUUID, v1dto.GetMyReviewsQuery{})

		require.NoError(t, err)
		assert.Equal(t, int32(0), total)
		assert.Empty(t, reviews)
	})

	t.Run("Case 3: Happy Path -> Returns MyReviewDTOs", func(t *testing.T) {
		mockUserRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return sqlc.User{UserID: 1, UserUuid: userUUID}, nil
			},
		}
		mockReviewRepo := &testutils.MockReviewRepository{
			CountByUserIDFunc: func(ctx context.Context, userID int32) (int64, error) {
				return 1, nil
			},
			GetByUserIDFunc: func(ctx context.Context, userID, limit, offset int32) ([]sqlc.GetReviewsByUserIDRow, error) {
				return []sqlc.GetReviewsByUserIDRow{
					{
						ReviewID:        1,
						ReviewUuid:      reviewUUID,
						ReviewRating:    5,
						ReviewTitle:     utils.StringToPointer("Awesome"),
						ReviewCreatedAt: time.Now(),
					},
				}, nil
			},
		}

		svc := v1service.NewReviewService(mockReviewRepo, mockUserRepo, nil, nil, nil, nil)
		reviews, total, err := svc.GetMyReviews(ctx, userUUID, v1dto.GetMyReviewsQuery{})

		require.NoError(t, err)
		assert.Equal(t, int32(1), total)
		assert.Len(t, reviews, 1)
		assert.Equal(t, reviewUUID.String(), reviews[0].UUID)
	})
}

func TestReviewService_UpdateReview_Eligibility(t *testing.T) {
	ctx := context.Background()
	userUUID := uuid.New()
	reviewUUID := uuid.New()

	t.Run("Case 1: Forbidden When Updating Others Review -> Returns 403", func(t *testing.T) {
		mockUserRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return sqlc.User{UserID: 1, UserUuid: userUUID}, nil
			},
		}
		mockReviewRepo := &testutils.MockReviewRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.GetReviewByUUIDRow, error) {
				return sqlc.GetReviewByUUIDRow{
					ReviewID:     10,
					ReviewUuid:   reviewUUID,
					ReviewUserID: 999, // Another user
				}, nil
			},
		}

		svc := v1service.NewReviewService(mockReviewRepo, mockUserRepo, nil, nil, nil, nil)
		_, err := svc.UpdateReview(ctx, userUUID, reviewUUID, v1dto.UpdateReviewInput{Rating: 4})

		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeForbidden)
	})

	t.Run("Case 2: Edit Count Exceeded (>= 1) -> Returns 400", func(t *testing.T) {
		mockUserRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return sqlc.User{UserID: 1, UserUuid: userUUID}, nil
			},
		}
		mockReviewRepo := &testutils.MockReviewRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.GetReviewByUUIDRow, error) {
				return sqlc.GetReviewByUUIDRow{
					ReviewID:        10,
					ReviewUuid:      reviewUUID,
					ReviewUserID:    1,
					ReviewEditCount: 1, // Already edited once
					ReviewCreatedAt: time.Now(),
				}, nil
			},
		}

		svc := v1service.NewReviewService(mockReviewRepo, mockUserRepo, nil, nil, nil, nil)
		_, err := svc.UpdateReview(ctx, userUUID, reviewUUID, v1dto.UpdateReviewInput{Rating: 4})

		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeBadRequest)
		assert.Contains(t, err.Error(), "only edit your review once")
	})

	t.Run("Case 3: Edit Time Exceeded (> 30 days) -> Returns 400", func(t *testing.T) {
		mockUserRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return sqlc.User{UserID: 1, UserUuid: userUUID}, nil
			},
		}
		mockReviewRepo := &testutils.MockReviewRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.GetReviewByUUIDRow, error) {
				return sqlc.GetReviewByUUIDRow{
					ReviewID:        10,
					ReviewUuid:      reviewUUID,
					ReviewUserID:    1,
					ReviewEditCount: 0,
					ReviewCreatedAt: time.Now().Add(-35 * 24 * time.Hour), // 35 days ago
				}, nil
			},
		}

		svc := v1service.NewReviewService(mockReviewRepo, mockUserRepo, nil, nil, nil, nil)
		_, err := svc.UpdateReview(ctx, userUUID, reviewUUID, v1dto.UpdateReviewInput{Rating: 4})

		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeBadRequest)
		assert.Contains(t, err.Error(), "within 30 days")
	})
}

func TestReviewService_DeleteReview_Authorization(t *testing.T) {
	ctx := context.Background()
	userUUID := uuid.New()
	reviewUUID := uuid.New()

	t.Run("Case 1: Forbidden When Deleting Others Review -> Returns 403", func(t *testing.T) {
		mockUserRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return sqlc.User{UserID: 1, UserUuid: userUUID}, nil
			},
		}
		mockReviewRepo := &testutils.MockReviewRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.GetReviewByUUIDRow, error) {
				return sqlc.GetReviewByUUIDRow{
					ReviewID:     10,
					ReviewUuid:   reviewUUID,
					ReviewUserID: 999, // Another user
				}, nil
			},
		}

		svc := v1service.NewReviewService(mockReviewRepo, mockUserRepo, nil, nil, nil, nil)
		err := svc.DeleteReview(ctx, userUUID, reviewUUID)

		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeForbidden)
	})
}

func TestReviewService_ToggleHelpfulVote(t *testing.T) {
	ctx := context.Background()
	userUUID := uuid.New()
	reviewUUID := uuid.New()

	t.Run("Case 1: Cannot Vote Own Review -> Returns 400", func(t *testing.T) {
		mockUserRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return sqlc.User{UserID: 1, UserUuid: userUUID}, nil
			},
		}
		mockReviewRepo := &testutils.MockReviewRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.GetReviewByUUIDRow, error) {
				return sqlc.GetReviewByUUIDRow{
					ReviewID:     10,
					ReviewUuid:   reviewUUID,
					ReviewUserID: 1, // Own review
				}, nil
			},
		}

		svc := v1service.NewReviewService(mockReviewRepo, mockUserRepo, nil, nil, nil, nil)
		_, err := svc.ToggleHelpfulVote(ctx, userUUID, reviewUUID)

		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeBadRequest)
		assert.Contains(t, err.Error(), "cannot vote for your own review")
	})
}

func TestReviewService_ReportReview(t *testing.T) {
	ctx := context.Background()
	userUUID := uuid.New()
	reviewUUID := uuid.New()

	t.Run("Case 1: Cannot Report Own Review -> Returns 400", func(t *testing.T) {
		mockUserRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return sqlc.User{UserID: 1, UserUuid: userUUID}, nil
			},
		}
		mockReviewRepo := &testutils.MockReviewRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.GetReviewByUUIDRow, error) {
				return sqlc.GetReviewByUUIDRow{
					ReviewID:     10,
					ReviewUuid:   reviewUUID,
					ReviewUserID: 1, // Own review
				}, nil
			},
		}

		svc := v1service.NewReviewService(mockReviewRepo, mockUserRepo, nil, nil, nil, nil)
		err := svc.ReportReview(ctx, userUUID, reviewUUID, v1dto.ReportReviewInput{Reason: "Spam"})

		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeBadRequest)
		assert.Contains(t, err.Error(), "cannot report your own review")
	})

	t.Run("Case 2: Already Reported -> Returns 400", func(t *testing.T) {
		mockUserRepo := &testutils.MockUserRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.User, error) {
				return sqlc.User{UserID: 1, UserUuid: userUUID}, nil
			},
		}
		mockReviewRepo := &testutils.MockReviewRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.GetReviewByUUIDRow, error) {
				return sqlc.GetReviewByUUIDRow{
					ReviewID:     10,
					ReviewUuid:   reviewUUID,
					ReviewUserID: 2,
				}, nil
			},
			HasReportedFunc: func(ctx context.Context, userID, reviewID int32) (bool, error) {
				return true, nil // Already reported
			},
		}

		svc := v1service.NewReviewService(mockReviewRepo, mockUserRepo, nil, nil, nil, nil)
		err := svc.ReportReview(ctx, userUUID, reviewUUID, v1dto.ReportReviewInput{Reason: "Spam"})

		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeBadRequest)
		assert.Contains(t, err.Error(), "already reported")
	})
}

func TestReviewService_AdminGetAllReviews(t *testing.T) {
	ctx := context.Background()
	productUUID := uuid.New()
	reviewUUID := uuid.New()

	t.Run("Case 1: Invalid Product UUID -> Returns 400", func(t *testing.T) {
		invalidUUID := "invalid-uuid"
		svc := v1service.NewReviewService(nil, nil, nil, nil, nil, nil)
		_, _, err := svc.AdminGetAllReviews(ctx, v1dto.AdminGetReviewsQuery{
			ProductUUID: &invalidUUID,
		})

		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeBadRequest)
	})

	t.Run("Case 2: Product Not Found -> Returns 404", func(t *testing.T) {
		pUUIDStr := productUUID.String()
		mockProductRepo := &testutils.MockProductRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.GetProductByUUIDRow, error) {
				return sqlc.GetProductByUUIDRow{}, pgx.ErrNoRows
			},
		}

		svc := v1service.NewReviewService(nil, nil, mockProductRepo, nil, nil, nil)
		_, _, err := svc.AdminGetAllReviews(ctx, v1dto.AdminGetReviewsQuery{
			ProductUUID: &pUUIDStr,
		})

		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeNotFound)
	})

	t.Run("Case 3: Zero Reviews -> Returns Empty Slice", func(t *testing.T) {
		mockReviewRepo := &testutils.MockReviewRepository{
			AdminCountFunc: func(ctx context.Context, status *int16, rating *int32, productID *int32, search *string) (int64, error) {
				return 0, nil
			},
		}

		svc := v1service.NewReviewService(mockReviewRepo, nil, nil, nil, nil, nil)
		rows, total, err := svc.AdminGetAllReviews(ctx, v1dto.AdminGetReviewsQuery{})

		require.NoError(t, err)
		assert.Equal(t, int32(0), total)
		assert.Empty(t, rows)
	})

	t.Run("Case 4: Happy Path -> Returns AdminReviewRow List", func(t *testing.T) {
		mockReviewRepo := &testutils.MockReviewRepository{
			AdminCountFunc: func(ctx context.Context, status *int16, rating *int32, productID *int32, search *string) (int64, error) {
				return 1, nil
			},
			AdminGetAllFunc: func(ctx context.Context, status *int16, rating *int32, pUUID, search *string, orderBy, sort string, limit, offset int32) ([]v1dto.AdminReviewRow, error) {
				return []v1dto.AdminReviewRow{
					{
						ReviewID:     1,
						ReviewUUID:   reviewUUID,
						ReviewRating: 5,
						ReviewStatus: 1,
						ReviewerName: "John Doe",
					},
				}, nil
			},
		}

		svc := v1service.NewReviewService(mockReviewRepo, nil, nil, nil, nil, nil)
		rows, total, err := svc.AdminGetAllReviews(ctx, v1dto.AdminGetReviewsQuery{})

		require.NoError(t, err)
		assert.Equal(t, int32(1), total)
		assert.Len(t, rows, 1)
		assert.Equal(t, reviewUUID, rows[0].ReviewUUID)
	})
}

func TestReviewService_AdminReplyReview(t *testing.T) {
	ctx := context.Background()
	reviewUUID := uuid.New()

	t.Run("Case 1: Review Not Found -> Returns 404", func(t *testing.T) {
		mockReviewRepo := &testutils.MockReviewRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.GetReviewByUUIDRow, error) {
				return sqlc.GetReviewByUUIDRow{}, pgx.ErrNoRows
			},
		}

		svc := v1service.NewReviewService(mockReviewRepo, nil, nil, nil, nil, nil)
		_, err := svc.AdminReplyReview(ctx, reviewUUID, v1dto.ReplyReviewInput{
			Reply: "Thank you for your feedback!",
		})

		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeNotFound)
	})
}

func TestReviewService_AdminForceDeleteReview(t *testing.T) {
	ctx := context.Background()
	reviewUUID := uuid.New()

	t.Run("Case 1: Review Not Found -> Returns 404", func(t *testing.T) {
		mockReviewRepo := &testutils.MockReviewRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (sqlc.GetReviewByUUIDRow, error) {
				return sqlc.GetReviewByUUIDRow{}, pgx.ErrNoRows
			},
		}

		svc := v1service.NewReviewService(mockReviewRepo, nil, nil, nil, nil, nil)
		err := svc.AdminForceDeleteReview(ctx, reviewUUID)

		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeNotFound)
	})
}
