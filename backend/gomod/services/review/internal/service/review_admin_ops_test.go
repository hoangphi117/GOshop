package service_test

import (
	"context"
	"testing"
	"time"

	catalogcontract "goshop/services/catalog/contract"
	reviewsqlc "goshop/services/review/internal/db/sqlc"
	reviewdto "goshop/services/review/internal/dto"
	reviewsvc "goshop/services/review/internal/service"
	"goshop/services/review/internal/service/servicetest"
	"goshop/shared/pkg/utils"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func sampleProductReview(reviewID int32, reviewUUID uuid.UUID, userID, productID int32) reviewsqlc.ProductReview {
	now := time.Now()
	title := "Awesome"
	comment := "Great quality product!"
	return reviewsqlc.ProductReview{
		ReviewID:         reviewID,
		ReviewUuid:       reviewUUID,
		ReviewUserID:     userID,
		ReviewProductID:  productID,
		ReviewRating:     5,
		ReviewTitle:      &title,
		ReviewComment:    &comment,
		ReviewStatus:     reviewsvc.ReviewStatusVisible,
		ReviewCreatedAt:  now,
		ReviewUpdatedAt:  now,
	}
}

func TestReviewService_AdminOpsAndLifecycle(t *testing.T) {
	ctx := context.Background()
	rUUID := uuid.New()
	pUUID := uuid.New()
	reviewID := int32(100)
	userID := int32(42)
	productID := int32(10)

	t.Run("AdminReplyReview happy path", func(t *testing.T) {
		replyText := "Thank you for shopping with us!"
		mockRepo := &servicetest.MockReviewRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (reviewsqlc.ProductReview, error) {
				return sampleProductReview(reviewID, rUUID, userID, productID), nil
			},
			ReplyFunc: func(ctx context.Context, id int32, reply string) (reviewsqlc.ProductReview, error) {
				sr := sampleProductReview(id, rUUID, userID, productID)
				sr.ReviewReply = &replyText
				now := time.Now()
				sr.ReviewReplyAt = pgtype.Timestamptz{Time: now, Valid: true}
				return sr, nil
			},
			GetRatingAggregateFunc: func(ctx context.Context, pid int32) (reviewsqlc.GetProductRatingAggregateRow, error) {
				return reviewsqlc.GetProductRatingAggregateRow{AvgRating: 5.0, ReviewCount: 1}, nil
			},
		}

		mockCatalog := &servicetest.MockProductCatalog{
			GetProductByUUIDFunc: func(ctx context.Context, u string) (*catalogcontract.ProductItemInfo, error) {
				return &catalogcontract.ProductItemInfo{ProductID: productID, ProductUUID: pUUID.String()}, nil
			},
		}

		svc := newService(mockRepo, mockCatalog, nil, nil)
		res, err := svc.AdminReplyReview(ctx, rUUID, reviewdto.ReplyReviewInput{Reply: replyText})
		require.NoError(t, err)
		assert.Equal(t, rUUID.String(), res.UUID)
		assert.NotNil(t, res.Reply)
		assert.Equal(t, replyText, *res.Reply)
	})

	t.Run("UpdateReviewStatus happy path and validation", func(t *testing.T) {
		mockRepo := &servicetest.MockReviewRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (reviewsqlc.ProductReview, error) {
				return sampleProductReview(reviewID, rUUID, userID, productID), nil
			},
			UpdateStatusFunc: func(ctx context.Context, id int32, status int16) (reviewsqlc.ProductReview, error) {
				sr := sampleProductReview(id, rUUID, userID, productID)
				sr.ReviewStatus = status
				return sr, nil
			},
			GetRatingAggregateFunc: func(ctx context.Context, pid int32) (reviewsqlc.GetProductRatingAggregateRow, error) {
				return reviewsqlc.GetProductRatingAggregateRow{AvgRating: 5.0, ReviewCount: 1}, nil
			},
		}

		mockCatalog := &servicetest.MockProductCatalog{
			GetProductByUUIDFunc: func(ctx context.Context, u string) (*catalogcontract.ProductItemInfo, error) {
				return &catalogcontract.ProductItemInfo{ProductID: productID, ProductUUID: pUUID.String()}, nil
			},
		}

		svc := newService(mockRepo, mockCatalog, nil, nil)

		// Invalid status
		_, err := svc.UpdateReviewStatus(ctx, rUUID, reviewdto.UpdateReviewStatusInput{Status: 99})
		require.Error(t, err)

		// Valid status: hidden
		res, err := svc.UpdateReviewStatus(ctx, rUUID, reviewdto.UpdateReviewStatusInput{Status: reviewsvc.ReviewStatusHidden})
		require.NoError(t, err)
		assert.Equal(t, rUUID.String(), res.UUID)
	})

	t.Run("AdminForceDeleteReview happy path", func(t *testing.T) {
		deleted := false
		mockRepo := &servicetest.MockReviewRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (reviewsqlc.ProductReview, error) {
				return sampleProductReview(reviewID, rUUID, userID, productID), nil
			},
			DeleteFunc: func(ctx context.Context, id int32) error {
				if id == reviewID {
					deleted = true
				}
				return nil
			},
			GetRatingAggregateFunc: func(ctx context.Context, pid int32) (reviewsqlc.GetProductRatingAggregateRow, error) {
				return reviewsqlc.GetProductRatingAggregateRow{AvgRating: 0.0, ReviewCount: 0}, nil
			},
		}

		svc := newService(mockRepo, nil, nil, nil)
		err := svc.AdminForceDeleteReview(ctx, rUUID)
		require.NoError(t, err)
		assert.True(t, deleted)
	})

	t.Run("AdminDismissReports happy path restores status to visible", func(t *testing.T) {
		reportsDeleted := false
		statusRestored := false
		sr := sampleProductReview(reviewID, rUUID, userID, productID)
		sr.ReviewStatus = reviewsvc.ReviewStatusReported

		mockRepo := &servicetest.MockReviewRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (reviewsqlc.ProductReview, error) {
				return sr, nil
			},
			DeleteReportsByReviewIDFunc: func(ctx context.Context, id int32) error {
				reportsDeleted = true
				return nil
			},
			UpdateStatusFunc: func(ctx context.Context, id int32, status int16) (reviewsqlc.ProductReview, error) {
				if status == reviewsvc.ReviewStatusVisible {
					statusRestored = true
				}
				sr.ReviewStatus = status
				return sr, nil
			},
			GetRatingAggregateFunc: func(ctx context.Context, pid int32) (reviewsqlc.GetProductRatingAggregateRow, error) {
				return reviewsqlc.GetProductRatingAggregateRow{AvgRating: 5.0, ReviewCount: 1}, nil
			},
		}

		svc := newService(mockRepo, nil, nil, nil)
		err := svc.AdminDismissReports(ctx, rUUID)
		require.NoError(t, err)
		assert.True(t, reportsDeleted)
		assert.True(t, statusRestored)
	})
}

func TestReviewService_UserInteractions(t *testing.T) {
	ctx := context.Background()
	rUUID := uuid.New()
	pUUID := uuid.New()
	reviewID := int32(100)
	authorID := int32(42)
	otherUserID := int32(99)
	productID := int32(10)

	t.Run("ToggleHelpfulVote cannot vote own review and toggles vote", func(t *testing.T) {
		voted := false
		mockRepo := &servicetest.MockReviewRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (reviewsqlc.ProductReview, error) {
				return sampleProductReview(reviewID, rUUID, authorID, productID), nil
			},
			HasHelpfulVoteFunc: func(ctx context.Context, uid, rid int32) (bool, error) {
				return voted, nil
			},
			AddHelpfulVoteFunc: func(ctx context.Context, uid, rid int32) (int32, error) {
				voted = true
				return 1, nil
			},
			RemoveHelpfulVoteFunc: func(ctx context.Context, uid, rid int32) (int32, error) {
				voted = false
				return 0, nil
			},
		}

		svc := newService(mockRepo, nil, nil, nil)

		// Author voting for own review -> 400
		_, err := svc.ToggleHelpfulVote(ctx, authorID, rUUID)
		require.Error(t, err)
		assert.Equal(t, utils.ErrCodeBadRequest, err.(*utils.AppError).Code)

		// Other user voting -> adds vote
		res1, err := svc.ToggleHelpfulVote(ctx, otherUserID, rUUID)
		require.NoError(t, err)
		assert.True(t, res1.Voted)
		assert.Equal(t, int32(1), res1.HelpfulCount)

		// Other user unvoting -> removes vote
		res2, err := svc.ToggleHelpfulVote(ctx, otherUserID, rUUID)
		require.NoError(t, err)
		assert.False(t, res2.Voted)
		assert.Equal(t, int32(0), res2.HelpfulCount)
	})

	t.Run("ReportReview validation and happy path", func(t *testing.T) {
		mockRepo := &servicetest.MockReviewRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (reviewsqlc.ProductReview, error) {
				return sampleProductReview(reviewID, rUUID, authorID, productID), nil
			},
			HasReportedFunc: func(ctx context.Context, uid, rid int32) (bool, error) {
				return false, nil
			},
			CreateReportFunc: func(ctx context.Context, params reviewsqlc.InsertReviewReportParams) error {
				return nil
			},
			CountReportsFunc: func(ctx context.Context, rid int32) (int64, error) {
				return 5, nil // auto-hide threshold
			},
			UpdateStatusFunc: func(ctx context.Context, rid int32, status int16) (reviewsqlc.ProductReview, error) {
				sr := sampleProductReview(rid, rUUID, authorID, productID)
				sr.ReviewStatus = status
				return sr, nil
			},
			GetRatingAggregateFunc: func(ctx context.Context, pid int32) (reviewsqlc.GetProductRatingAggregateRow, error) {
				return reviewsqlc.GetProductRatingAggregateRow{AvgRating: 4.0, ReviewCount: 1}, nil
			},
		}

		svc := newService(mockRepo, nil, nil, nil)

		// Author reporting self -> error
		err := svc.ReportReview(ctx, authorID, rUUID, reviewdto.ReportReviewInput{Reason: "spam"})
		require.Error(t, err)

		// Invalid reason -> error
		err = svc.ReportReview(ctx, otherUserID, rUUID, reviewdto.ReportReviewInput{Reason: "unknown_reason"})
		require.Error(t, err)

		// Valid report -> success and auto-hides
		err = svc.ReportReview(ctx, otherUserID, rUUID, reviewdto.ReportReviewInput{Reason: "spam"})
		require.NoError(t, err)
	})

	t.Run("DeleteReview checks owner permission", func(t *testing.T) {
		deleted := false
		mockRepo := &servicetest.MockReviewRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (reviewsqlc.ProductReview, error) {
				return sampleProductReview(reviewID, rUUID, authorID, productID), nil
			},
			DeleteFunc: func(ctx context.Context, id int32) error {
				deleted = true
				return nil
			},
			GetRatingAggregateFunc: func(ctx context.Context, pid int32) (reviewsqlc.GetProductRatingAggregateRow, error) {
				return reviewsqlc.GetProductRatingAggregateRow{AvgRating: 0.0, ReviewCount: 0}, nil
			},
		}

		svc := newService(mockRepo, nil, nil, nil)

		// Non-owner -> 403 Forbidden
		err := svc.DeleteReview(ctx, otherUserID, rUUID)
		require.Error(t, err)
		assert.Equal(t, utils.ErrCodeForbidden, err.(*utils.AppError).Code)

		// Owner -> Success
		err = svc.DeleteReview(ctx, authorID, rUUID)
		require.NoError(t, err)
		assert.True(t, deleted)
	})

	t.Run("UpdateReview checks owner permission and updates", func(t *testing.T) {
		mockRepo := &servicetest.MockReviewRepository{
			GetByUUIDFunc: func(ctx context.Context, u uuid.UUID) (reviewsqlc.ProductReview, error) {
				return sampleProductReview(reviewID, rUUID, authorID, productID), nil
			},
			UpdateFunc: func(ctx context.Context, params reviewsqlc.UpdateReviewParams) (reviewsqlc.ProductReview, error) {
				sr := sampleProductReview(reviewID, rUUID, authorID, productID)
				sr.ReviewRating = params.ReviewRating
				sr.ReviewTitle = params.ReviewTitle
				sr.ReviewComment = params.ReviewComment
				return sr, nil
			},
			GetRatingAggregateFunc: func(ctx context.Context, pid int32) (reviewsqlc.GetProductRatingAggregateRow, error) {
				return reviewsqlc.GetProductRatingAggregateRow{AvgRating: 4.0, ReviewCount: 1}, nil
			},
		}

		svc := newService(mockRepo, nil, nil, nil)

		// Non-owner -> 403
		title := "Updated Title"
		comment := "Updated comment text"
		_, err := svc.UpdateReview(ctx, otherUserID, rUUID, reviewdto.UpdateReviewInput{
			Rating:  4,
			Title:   &title,
			Comment: &comment,
		})
		require.Error(t, err)

		// Owner -> Success
		res, err := svc.UpdateReview(ctx, authorID, rUUID, reviewdto.UpdateReviewInput{
			Rating:  4,
			Title:   &title,
			Comment: &comment,
		})
		require.NoError(t, err)
		assert.Equal(t, int32(4), res.Rating)
		assert.Equal(t, &title, res.Title)
	})

	t.Run("GetProductRatingSummary happy path", func(t *testing.T) {
		mockCatalog := &servicetest.MockProductCatalog{
			GetProductByUUIDFunc: func(ctx context.Context, u string) (*catalogcontract.ProductItemInfo, error) {
				return &catalogcontract.ProductItemInfo{ProductID: productID, ProductUUID: pUUID.String()}, nil
			},
		}
		mockRepo := &servicetest.MockReviewRepository{
			GetRatingSummaryFunc: func(ctx context.Context, pid int32) (reviewsqlc.GetProductRatingSummaryRow, error) {
				return reviewsqlc.GetProductRatingSummaryRow{
					TotalReviews: 10,
					AvgRating:    4.5,
					FiveStar:     6,
					FourStar:     3,
					ThreeStar:    1,
				}, nil
			},
		}

		svc := newService(mockRepo, mockCatalog, nil, nil)
		summary, err := svc.GetProductRatingSummary(ctx, pUUID)
		require.NoError(t, err)
		assert.Equal(t, int64(10), summary.TotalReviews)
		assert.Equal(t, 4.5, summary.AvgRating)
	})
}
