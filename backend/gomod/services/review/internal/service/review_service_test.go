package service_test

import (
	"context"
	"testing"
	"time"

	catalogcontract "goshop/services/catalog/contract"
	ordercontract "goshop/services/order/contract"
	reviewsqlc "goshop/services/review/internal/db/sqlc"
	reviewdto "goshop/services/review/internal/dto"
	reviewsvc "goshop/services/review/internal/service"
	"goshop/services/review/internal/service/servicetest"
	"goshop/shared/pkg/utils"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newService(repo *servicetest.MockReviewRepository, catalog *servicetest.MockProductCatalog, verifier *servicetest.MockPurchaseVerifier, userReader *servicetest.MockUserReader) reviewsvc.ReviewService {
	var cat reviewsvc.ProductCatalog
	if catalog != nil {
		cat = catalog
	}
	var ver reviewsvc.PurchaseVerifier
	if verifier != nil {
		ver = verifier
	}
	var usr reviewsvc.UserReader
	if userReader != nil {
		usr = userReader
	}
	return reviewsvc.NewReviewService(repo, cat, ver, usr, nil, nil, noopTxManager{})
}

func productCatalogMock(productID int32, productUUID uuid.UUID, reviewCount int32) *servicetest.MockProductCatalog {
	return &servicetest.MockProductCatalog{
		GetProductByUUIDFunc: func(ctx context.Context, u string) (*catalogcontract.ProductItemInfo, error) {
			if u != productUUID.String() {
				return nil, pgx.ErrNoRows
			}
			return &catalogcontract.ProductItemInfo{
				ProductID:          productID,
				ProductUUID:        productUUID.String(),
				ProductReviewCount: reviewCount,
			}, nil
		},
	}
}

func verifierMock(isPurchased, canReview bool) *servicetest.MockPurchaseVerifier {
	return &servicetest.MockPurchaseVerifier{
		VerifyPurchaseFunc: func(ctx context.Context, oUUID uuid.UUID, uID, pID int32, _, _ *uuid.UUID) (*ordercontract.PurchaseVerification, error) {
			return &ordercontract.PurchaseVerification{
				OrderID:      100,
				OrderUUID:    oUUID,
				UserID:       uID,
				IsPurchased:  isPurchased,
				CanReview:    canReview,
				CustomerName: "John Doe",
			}, nil
		},
	}
}

func TestReviewService_CreateReview(t *testing.T) {
	ctx := context.Background()
	productUUID := uuid.New()
	orderUUID := uuid.New()
	catalog := productCatalogMock(10, productUUID, 0)

	t.Run("Case 1: Invalid Product UUID -> Returns 400", func(t *testing.T) {
		svc := newService(nil, nil, nil, nil)
		_, err := svc.CreateReview(ctx, int32(1), reviewdto.CreateReviewInput{
			ProductUUID: "invalid-uuid",
			OrderUUID:   orderUUID.String(),
			Rating:      5,
		}, nil, nil)

		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeBadRequest)
	})

	t.Run("Case 2: Product Not Found -> Returns 404", func(t *testing.T) {
		svc := newService(nil, productCatalogMock(10, uuid.New(), 0), nil, nil)
		_, err := svc.CreateReview(ctx, int32(1), reviewdto.CreateReviewInput{
			ProductUUID: productUUID.String(),
			OrderUUID:   orderUUID.String(),
			Rating:      5,
		}, nil, nil)

		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeNotFound)
	})

	t.Run("Case 3: User Has Not Purchased Product -> Returns 400", func(t *testing.T) {
		mockReviewRepo := &servicetest.MockReviewRepository{}

		svc := newService(mockReviewRepo, catalog, verifierMock(false, true), nil)
		_, err := svc.CreateReview(ctx, int32(1), reviewdto.CreateReviewInput{
			ProductUUID: productUUID.String(),
			OrderUUID:   orderUUID.String(),
			Rating:      5,
		}, nil, nil)

		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeBadRequest)
		assert.Contains(t, err.Error(), "not purchased")
	})

	t.Run("Case 4: Review Already Exists -> Returns 400", func(t *testing.T) {
		mockReviewRepo := &servicetest.MockReviewRepository{
			CheckExistsFunc: func(ctx context.Context, userID, productID int32, oUUID uuid.UUID) (bool, error) {
				return true, nil // Already reviewed
			},
		}

		svc := newService(mockReviewRepo, catalog, verifierMock(true, true), nil)
		_, err := svc.CreateReview(ctx, int32(1), reviewdto.CreateReviewInput{
			ProductUUID: productUUID.String(),
			OrderUUID:   orderUUID.String(),
			Rating:      5,
		}, nil, nil)

		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeBadRequest)
		assert.Contains(t, err.Error(), "already exists")
	})

	t.Run("Case 5: Review Window Expired (> 30 days after delivery) -> Returns 400", func(t *testing.T) {
		svc := newService(&servicetest.MockReviewRepository{}, catalog, verifierMock(true, false), nil)
		_, err := svc.CreateReview(ctx, int32(1), reviewdto.CreateReviewInput{
			ProductUUID: productUUID.String(),
			OrderUUID:   orderUUID.String(),
			Rating:      5,
		}, nil, nil)

		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeBadRequest)
		assert.Contains(t, err.Error(), "expired")
	})
}

func TestReviewService_GetProductReviews(t *testing.T) {
	ctx := context.Background()
	productUUID := uuid.New()
	reviewUUID := uuid.New()

	t.Run("Case 1: Product Not Found -> Returns 404", func(t *testing.T) {
		svc := newService(nil, productCatalogMock(10, uuid.New(), 0), nil, nil)
		_, _, err := svc.GetProductReviews(ctx, productUUID, reviewdto.GetReviewsQuery{})

		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeNotFound)
	})

	t.Run("Case 2: Happy Path -> Returns Reviews List and Total", func(t *testing.T) {
		catalog := &servicetest.MockProductCatalog{
			GetProductByUUIDFunc: func(ctx context.Context, u string) (*catalogcontract.ProductItemInfo, error) {
				return &catalogcontract.ProductItemInfo{
					ProductID:          10,
					ProductUUID:        productUUID.String(),
					ProductReviewCount: 1,
				}, nil
			},
		}
		mockReviewRepo := &servicetest.MockReviewRepository{
			// Total must come from the review DB, not the catalog aggregate.
			CountByProductFunc: func(ctx context.Context, productID int32, rating *int32, hasMedia *bool) (int64, error) {
				return 1, nil
			},
			GetByProductFunc: func(ctx context.Context, productID int32, rating *int32, hasMedia *bool, orderBy, sort string, limit, offset int32) ([]reviewdto.ReviewRow, error) {
				return []reviewdto.ReviewRow{
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

		svc := newService(mockReviewRepo, catalog, nil, nil)
		reviews, total, err := svc.GetProductReviews(ctx, productUUID, reviewdto.GetReviewsQuery{})

		require.NoError(t, err)
		assert.Equal(t, int32(1), total)
		assert.Len(t, reviews, 1)
		assert.Equal(t, reviewUUID.String(), reviews[0].UUID)
		assert.Equal(t, int32(5), reviews[0].Rating)
	})

	t.Run("Case 3: Total Comes From Review DB, Not Catalog Aggregate", func(t *testing.T) {
		// Catalog's denormalized count (99) lags behind; the DB is authoritative.
		catalog := productCatalogMock(10, productUUID, 99)
		mockReviewRepo := &servicetest.MockReviewRepository{
			CountByProductFunc: func(ctx context.Context, productID int32, rating *int32, hasMedia *bool) (int64, error) {
				return 2, nil
			},
			GetByProductFunc: func(ctx context.Context, productID int32, rating *int32, hasMedia *bool, orderBy, sort string, limit, offset int32) ([]reviewdto.ReviewRow, error) {
				return nil, nil
			},
		}

		svc := newService(mockReviewRepo, catalog, nil, nil)
		_, total, err := svc.GetProductReviews(ctx, productUUID, reviewdto.GetReviewsQuery{})

		require.NoError(t, err)
		assert.Equal(t, int32(2), total)
	})
}

func TestReviewService_GetProductRatingSummary(t *testing.T) {
	ctx := context.Background()
	productUUID := uuid.New()

	t.Run("Case 1: Product Not Found -> Returns 404", func(t *testing.T) {
		svc := newService(nil, productCatalogMock(10, uuid.New(), 0), nil, nil)
		_, err := svc.GetProductRatingSummary(ctx, productUUID)

		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeNotFound)
	})

	t.Run("Case 2: Happy Path -> Returns Rating Summary", func(t *testing.T) {
		catalog := productCatalogMock(10, productUUID, 0)
		mockReviewRepo := &servicetest.MockReviewRepository{
			GetRatingSummaryFunc: func(ctx context.Context, productID int32) (reviewsqlc.GetProductRatingSummaryRow, error) {
				return reviewsqlc.GetProductRatingSummaryRow{
					TotalReviews:   10,
					AvgRating:      4.8,
					WithMediaCount: 4,
					FiveStar:       8,
					FourStar:       2,
				}, nil
			},
		}

		svc := newService(mockReviewRepo, catalog, nil, nil)
		summary, err := svc.GetProductRatingSummary(ctx, productUUID)

		require.NoError(t, err)
		assert.Equal(t, int64(10), summary.TotalReviews)
		assert.InDelta(t, 4.8, summary.AvgRating, 0.001)
		assert.Equal(t, int64(8), summary.FiveStar)
	})
}

func TestReviewService_GetMyReviews(t *testing.T) {
	ctx := context.Background()
	reviewUUID := uuid.New()
	productUUID := uuid.New()

	t.Run("Case 1: Zero Reviews -> Returns Empty Slice", func(t *testing.T) {
		mockReviewRepo := &servicetest.MockReviewRepository{
			CountByUserIDFunc: func(ctx context.Context, userID int32) (int64, error) {
				return 0, nil
			},
		}

		svc := newService(mockReviewRepo, nil, nil, nil)
		reviews, total, err := svc.GetMyReviews(ctx, int32(1), reviewdto.GetMyReviewsQuery{})

		require.NoError(t, err)
		assert.Equal(t, int32(0), total)
		assert.Empty(t, reviews)
	})

	t.Run("Case 2: Happy Path -> Returns MyReviewDTOs", func(t *testing.T) {
		mockReviewRepo := &servicetest.MockReviewRepository{
			CountByUserIDFunc: func(ctx context.Context, userID int32) (int64, error) {
				return 1, nil
			},
			GetByUserIDFunc: func(ctx context.Context, userID, limit, offset int32) ([]reviewsqlc.GetReviewsByUserIDRow, error) {
				return []reviewsqlc.GetReviewsByUserIDRow{
					{
						ReviewID:          1,
						ReviewUuid:        reviewUUID,
						ReviewProductUuid: productUUID,
						ReviewRating:      5,
						ReviewTitle:       utils.StringToPointer("Awesome"),
						ReviewCreatedAt:   time.Now(),
					},
				}, nil
			},
		}

		svc := newService(mockReviewRepo, nil, nil, nil)
		reviews, total, err := svc.GetMyReviews(ctx, int32(1), reviewdto.GetMyReviewsQuery{})

		require.NoError(t, err)
		assert.Equal(t, int32(1), total)
		assert.Len(t, reviews, 1)
		assert.Equal(t, reviewUUID.String(), reviews[0].UUID)
	})
}

func TestReviewService_UpdateReview_Eligibility(t *testing.T) {
	ctx := context.Background()
	reviewUUID := uuid.New()

	t.Run("Case 1: Forbidden When Updating Others Review -> Returns 403", func(t *testing.T) {
		mockReviewRepo := &servicetest.MockReviewRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (reviewsqlc.ProductReview, error) {
				return reviewsqlc.ProductReview{
					ReviewID:     10,
					ReviewUuid:   reviewUUID,
					ReviewUserID: 999, // Another user
				}, nil
			},
		}

		svc := newService(mockReviewRepo, nil, nil, nil)
		_, err := svc.UpdateReview(ctx, int32(1), reviewUUID, reviewdto.UpdateReviewInput{Rating: 4})

		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeForbidden)
	})

	t.Run("Case 2: Edit Count Exceeded (>= 1) -> Returns 400", func(t *testing.T) {
		mockReviewRepo := &servicetest.MockReviewRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (reviewsqlc.ProductReview, error) {
				return reviewsqlc.ProductReview{
					ReviewID:        10,
					ReviewUuid:      reviewUUID,
					ReviewUserID:    1,
					ReviewEditCount: 1, // Already edited once
					ReviewCreatedAt: time.Now(),
				}, nil
			},
		}

		svc := newService(mockReviewRepo, nil, nil, nil)
		_, err := svc.UpdateReview(ctx, int32(1), reviewUUID, reviewdto.UpdateReviewInput{Rating: 4})

		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeBadRequest)
		assert.Contains(t, err.Error(), "only edit your review once")
	})

	t.Run("Case 3: Edit Time Exceeded (> 30 days) -> Returns 400", func(t *testing.T) {
		mockReviewRepo := &servicetest.MockReviewRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (reviewsqlc.ProductReview, error) {
				return reviewsqlc.ProductReview{
					ReviewID:        10,
					ReviewUuid:      reviewUUID,
					ReviewUserID:    1,
					ReviewEditCount: 0,
					ReviewCreatedAt: time.Now().Add(-35 * 24 * time.Hour), // 35 days ago
				}, nil
			},
		}

		svc := newService(mockReviewRepo, nil, nil, nil)
		_, err := svc.UpdateReview(ctx, int32(1), reviewUUID, reviewdto.UpdateReviewInput{Rating: 4})

		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeBadRequest)
		assert.Contains(t, err.Error(), "within 30 days")
	})
}

func TestReviewService_DeleteReview_Authorization(t *testing.T) {
	ctx := context.Background()
	reviewUUID := uuid.New()

	t.Run("Case 1: Forbidden When Deleting Others Review -> Returns 403", func(t *testing.T) {
		mockReviewRepo := &servicetest.MockReviewRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (reviewsqlc.ProductReview, error) {
				return reviewsqlc.ProductReview{
					ReviewID:     10,
					ReviewUuid:   reviewUUID,
					ReviewUserID: 999, // Another user
				}, nil
			},
		}

		svc := newService(mockReviewRepo, nil, nil, nil)
		err := svc.DeleteReview(ctx, int32(1), reviewUUID)

		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeForbidden)
	})
}

func TestReviewService_ToggleHelpfulVote(t *testing.T) {
	ctx := context.Background()
	reviewUUID := uuid.New()

	t.Run("Case 1: Cannot Vote Own Review -> Returns 400", func(t *testing.T) {
		mockReviewRepo := &servicetest.MockReviewRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (reviewsqlc.ProductReview, error) {
				return reviewsqlc.ProductReview{
					ReviewID:     10,
					ReviewUuid:   reviewUUID,
					ReviewUserID: 1, // Own review
				}, nil
			},
		}

		svc := newService(mockReviewRepo, nil, nil, nil)
		_, err := svc.ToggleHelpfulVote(ctx, int32(1), reviewUUID)

		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeBadRequest)
		assert.Contains(t, err.Error(), "cannot vote for your own review")
	})
}

func TestReviewService_ReportReview(t *testing.T) {
	ctx := context.Background()
	reviewUUID := uuid.New()

	t.Run("Case 1: Cannot Report Own Review -> Returns 400", func(t *testing.T) {
		mockReviewRepo := &servicetest.MockReviewRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (reviewsqlc.ProductReview, error) {
				return reviewsqlc.ProductReview{
					ReviewID:     10,
					ReviewUuid:   reviewUUID,
					ReviewUserID: 1, // Own review
				}, nil
			},
		}

		svc := newService(mockReviewRepo, nil, nil, nil)
		err := svc.ReportReview(ctx, int32(1), reviewUUID, reviewdto.ReportReviewInput{Reason: "spam"})

		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeBadRequest)
		assert.Contains(t, err.Error(), "cannot report your own review")
	})

	t.Run("Case 2: Already Reported -> Returns 400", func(t *testing.T) {
		mockReviewRepo := &servicetest.MockReviewRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (reviewsqlc.ProductReview, error) {
				return reviewsqlc.ProductReview{
					ReviewID:     10,
					ReviewUuid:   reviewUUID,
					ReviewUserID: 2,
				}, nil
			},
			HasReportedFunc: func(ctx context.Context, userID, reviewID int32) (bool, error) {
				return true, nil // Already reported
			},
		}

		svc := newService(mockReviewRepo, nil, nil, nil)
		err := svc.ReportReview(ctx, int32(1), reviewUUID, reviewdto.ReportReviewInput{Reason: "spam"})

		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeBadRequest)
		assert.Contains(t, err.Error(), "already reported")
	})
}

func TestReviewService_AdminGetAllReviews(t *testing.T) {
	ctx := context.Background()
	reviewUUID := uuid.New()

	t.Run("Case 1: Invalid Product UUID -> Returns 400", func(t *testing.T) {
		invalidUUID := "invalid-uuid"
		svc := newService(nil, nil, nil, nil)
		_, _, err := svc.AdminGetAllReviews(ctx, reviewdto.AdminGetReviewsQuery{
			ProductUUID: &invalidUUID,
		})

		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeBadRequest)
	})

	t.Run("Case 2: Zero Reviews -> Returns Empty Slice", func(t *testing.T) {
		mockReviewRepo := &servicetest.MockReviewRepository{
			AdminCountFunc: func(ctx context.Context, status *int16, rating *int32, productUUID *uuid.UUID, search *string) (int64, error) {
				return 0, nil
			},
		}

		svc := newService(mockReviewRepo, nil, nil, nil)
		rows, total, err := svc.AdminGetAllReviews(ctx, reviewdto.AdminGetReviewsQuery{})

		require.NoError(t, err)
		assert.Equal(t, int32(0), total)
		assert.Empty(t, rows)
	})

	t.Run("Case 3: Happy Path -> Returns AdminReviewDTO List", func(t *testing.T) {
		adminProductUUID := uuid.New()
		mockReviewRepo := &servicetest.MockReviewRepository{
			AdminCountFunc: func(ctx context.Context, status *int16, rating *int32, productUUID *uuid.UUID, search *string) (int64, error) {
				return 1, nil
			},
			AdminGetAllFunc: func(ctx context.Context, status *int16, rating *int32, productUUID *uuid.UUID, search *string, orderBy, sort string, limit, offset int32) ([]reviewdto.AdminReviewRow, error) {
				return []reviewdto.AdminReviewRow{
					{
						ReviewID:     1,
						ReviewUUID:   reviewUUID,
						ProductUUID:  adminProductUUID,
						ReviewRating: 5,
						ReviewStatus: 1,
						ReviewerName: "John Doe",
					},
				}, nil
			},
		}

		svc := newService(mockReviewRepo, nil, nil, nil)
		rows, total, err := svc.AdminGetAllReviews(ctx, reviewdto.AdminGetReviewsQuery{})

		require.NoError(t, err)
		assert.Equal(t, int32(1), total)
		assert.Len(t, rows, 1)
		assert.Equal(t, reviewUUID.String(), rows[0].UUID)
	})
}

func TestReviewService_AdminReplyReview(t *testing.T) {
	ctx := context.Background()
	reviewUUID := uuid.New()

	t.Run("Case 1: Review Not Found -> Returns 404", func(t *testing.T) {
		mockReviewRepo := &servicetest.MockReviewRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (reviewsqlc.ProductReview, error) {
				return reviewsqlc.ProductReview{}, pgx.ErrNoRows
			},
		}

		svc := newService(mockReviewRepo, nil, nil, nil)
		_, err := svc.AdminReplyReview(ctx, reviewUUID, reviewdto.ReplyReviewInput{
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
		mockReviewRepo := &servicetest.MockReviewRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (reviewsqlc.ProductReview, error) {
				return reviewsqlc.ProductReview{}, pgx.ErrNoRows
			},
		}

		svc := newService(mockReviewRepo, nil, nil, nil)
		err := svc.AdminForceDeleteReview(ctx, reviewUUID)

		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeNotFound)
	})
}

func TestReviewService_UpdateReviewStatus(t *testing.T) {
	ctx := context.Background()
	reviewUUID := uuid.New()

	t.Run("Case 1: Review Not Found -> Returns 404", func(t *testing.T) {
		mockReviewRepo := &servicetest.MockReviewRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (reviewsqlc.ProductReview, error) {
				return reviewsqlc.ProductReview{}, pgx.ErrNoRows
			},
		}

		svc := newService(mockReviewRepo, nil, nil, nil)
		_, err := svc.UpdateReviewStatus(ctx, reviewUUID, reviewdto.UpdateReviewStatusInput{
			Status: 2,
		})

		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeNotFound)
	})
}

func TestReviewService_GetReviewReports(t *testing.T) {
	ctx := context.Background()
	reviewUUID := uuid.New()

	t.Run("Case 1: Review Not Found -> Returns 404", func(t *testing.T) {
		mockReviewRepo := &servicetest.MockReviewRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (reviewsqlc.ProductReview, error) {
				return reviewsqlc.ProductReview{}, pgx.ErrNoRows
			},
		}

		svc := newService(mockReviewRepo, nil, nil, nil)
		_, err := svc.GetReviewReports(ctx, reviewUUID)

		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeNotFound)
	})

	t.Run("Case 2: Happy Path -> Returns Mapped Reports", func(t *testing.T) {
		mockReviewRepo := &servicetest.MockReviewRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (reviewsqlc.ProductReview, error) {
				return reviewsqlc.ProductReview{
					ReviewID:   10,
					ReviewUuid: reviewUUID,
				}, nil
			},
			GetReportsByReviewIDFunc: func(ctx context.Context, reviewID int32) ([]reviewsqlc.GetReviewReportsByReviewIDRow, error) {
				return []reviewsqlc.GetReviewReportsByReviewIDRow{
					{
						ReportID:  1,
						UserID:    5,
						UserName:  "Spam Reporter",
						UserEmail: "reporter@example.com",
						Reason:    "spam",
						CreatedAt: time.Now(),
					},
				}, nil
			},
		}

		svc := newService(mockReviewRepo, nil, nil, nil)
		reports, err := svc.GetReviewReports(ctx, reviewUUID)

		require.NoError(t, err)
		assert.Len(t, reports, 1)
		assert.Equal(t, int32(1), reports[0].ReportID)
		assert.Equal(t, "Spam Reporter", reports[0].UserName)
		assert.Equal(t, "spam", reports[0].Reason)
	})
}

func TestReviewService_AdminDismissReports(t *testing.T) {
	ctx := context.Background()
	reviewUUID := uuid.New()

	t.Run("Case 1: Review Not Found -> Returns 404", func(t *testing.T) {
		mockReviewRepo := &servicetest.MockReviewRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (reviewsqlc.ProductReview, error) {
				return reviewsqlc.ProductReview{}, pgx.ErrNoRows
			},
		}

		svc := newService(mockReviewRepo, nil, nil, nil)
		err := svc.AdminDismissReports(ctx, reviewUUID)

		require.Error(t, err)
		assertAppErrorCode(t, err, utils.ErrCodeNotFound)
	})
}
