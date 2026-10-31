package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	catalogsqlc "goshop/services/catalog/internal/db/sqlc"
	"goshop/services/catalog/internal/dto"
	"goshop/services/catalog/internal/repository"
	inventorycontract "goshop/services/inventory/contract"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// mockRepo embeds the full interface so only the used methods need overrides.
type mockRepo struct {
	repository.CatalogRepository

	getCategoryByUUID func(ctx context.Context, u uuid.UUID) (catalogsqlc.Category, error)
	getProductBySlug  func(ctx context.Context, slug string) (catalogsqlc.GetProductBySlugRow, error)
	createProduct     func(ctx context.Context, p catalogsqlc.CreateProductParams) (catalogsqlc.CreateProductRow, error)
	getProductByUUID  func(ctx context.Context, u uuid.UUID) (catalogsqlc.GetProductByUUIDRow, error)
	insertOutboxEvent func(ctx context.Context, aggregateType, aggregateID, eventType string, payload any) (catalogsqlc.OutboxEvent, error)
	softDeleteProduct func(ctx context.Context, u uuid.UUID) (catalogsqlc.Product, error)
	softDeleteCalls   int
	getPendingOutbox  func(ctx context.Context, limit int32) ([]catalogsqlc.OutboxEvent, error)
	markProcessed     func(ctx context.Context, eventID int64) error
	markedEventIDs    []int64
}

func (m *mockRepo) GetCategoryByUUID(ctx context.Context, u uuid.UUID) (catalogsqlc.Category, error) {
	return m.getCategoryByUUID(ctx, u)
}

func (m *mockRepo) GetProductBySlug(ctx context.Context, slug string) (catalogsqlc.GetProductBySlugRow, error) {
	return m.getProductBySlug(ctx, slug)
}

func (m *mockRepo) CreateProduct(ctx context.Context, p catalogsqlc.CreateProductParams) (catalogsqlc.CreateProductRow, error) {
	return m.createProduct(ctx, p)
}

func (m *mockRepo) GetProductByUUID(ctx context.Context, u uuid.UUID) (catalogsqlc.GetProductByUUIDRow, error) {
	return m.getProductByUUID(ctx, u)
}

func (m *mockRepo) InsertOutboxEvent(ctx context.Context, aggregateType, aggregateID, eventType string, payload any) (catalogsqlc.OutboxEvent, error) {
	return m.insertOutboxEvent(ctx, aggregateType, aggregateID, eventType, payload)
}

func (m *mockRepo) SoftDeleteProduct(ctx context.Context, u uuid.UUID) (catalogsqlc.Product, error) {
	m.softDeleteCalls++
	return m.softDeleteProduct(ctx, u)
}

func (m *mockRepo) GetPendingOutboxEvents(ctx context.Context, limit int32) ([]catalogsqlc.OutboxEvent, error) {
	if m.getPendingOutbox == nil {
		return nil, nil
	}
	return m.getPendingOutbox(ctx, limit)
}

func (m *mockRepo) MarkOutboxEventProcessed(ctx context.Context, eventID int64) error {
	m.markedEventIDs = append(m.markedEventIDs, eventID)
	if m.markProcessed == nil {
		return nil
	}
	return m.markProcessed(ctx, eventID)
}

type mockInventory struct {
	inventorycontract.InventoryClient

	adjustErr   error
	adjustCalls int
	lastAdjust  inventorycontract.AdjustStockInput
}

func (m *mockInventory) AdjustStock(_ context.Context, input inventorycontract.AdjustStockInput) (*inventorycontract.AdjustStockResult, error) {
	m.adjustCalls++
	m.lastAdjust = input
	if m.adjustErr != nil {
		return nil, m.adjustErr
	}
	return &inventorycontract.AdjustStockResult{Success: true, CurrentStock: input.QuantityChange}, nil
}

func (m *mockInventory) GetStock(context.Context, []string, []int32) ([]inventorycontract.StockItemInfo, error) {
	return nil, errors.New("not expected in create workflow")
}

func newWorkflowService(repo repository.CatalogRepository, inv inventorycontract.InventoryClient) *catalogService {
	return &catalogService{repo: repo, inventoryClient: inv}
}

func stockRowFixture(u uuid.UUID) catalogsqlc.CreateProductRow {
	id := int32(7)
	return catalogsqlc.CreateProductRow{ProductID: id, ProductUuid: u, ProductName: "P", ProductSlug: "p"}
}

// TestCreateProduct_StockInitializedWithIdempotentRef verifies the initial
// stock call is explicit, uses the product UUID as the idempotency reference,
// and that the returned DTO skips the extra inventory read.
func TestCreateProduct_StockInitializedWithIdempotentRef(t *testing.T) {
	productUUID := uuid.New()
	inv := &mockInventory{}
	repo := &mockRepo{
		getCategoryByUUID: func(context.Context, uuid.UUID) (catalogsqlc.Category, error) {
			return catalogsqlc.Category{CategoryID: 3}, nil
		},
		getProductBySlug: func(context.Context, string) (catalogsqlc.GetProductBySlugRow, error) {
			return catalogsqlc.GetProductBySlugRow{}, pgx.ErrNoRows
		},
		createProduct: func(_ context.Context, _ catalogsqlc.CreateProductParams) (catalogsqlc.CreateProductRow, error) {
			return stockRowFixture(productUUID), nil
		},
		getProductByUUID: func(context.Context, uuid.UUID) (catalogsqlc.GetProductByUUIDRow, error) {
			return catalogsqlc.GetProductByUUIDRow{}, pgx.ErrNoRows
		},
		insertOutboxEvent: func(_ context.Context, _, _, _ string, _ any) (catalogsqlc.OutboxEvent, error) {
			return catalogsqlc.OutboxEvent{}, nil
		},
		softDeleteProduct: func(_ context.Context, _ uuid.UUID) (catalogsqlc.Product, error) {
			return catalogsqlc.Product{}, nil
		},
	}

	svc := newWorkflowService(repo, inv)
	res, err := svc.CreateProduct(context.Background(), dto.CreateProductInput{
		Name:         "P",
		CategoryUUID: uuid.NewString(),
		Stock:        25,
	})
	if err != nil {
		t.Fatalf("CreateProduct failed: %v", err)
	}
	if inv.adjustCalls != 1 {
		t.Fatalf("expected 1 AdjustStock call, got %d", inv.adjustCalls)
	}
	if inv.lastAdjust.ChangeType != "INITIAL_STOCK" {
		t.Fatalf("expected INITIAL_STOCK, got %s", inv.lastAdjust.ChangeType)
	}
	if inv.lastAdjust.QuantityChange != 25 {
		t.Fatalf("expected quantity 25, got %d", inv.lastAdjust.QuantityChange)
	}
	if inv.lastAdjust.ReferenceID == nil || *inv.lastAdjust.ReferenceID != productUUID.String() {
		t.Fatalf("expected reference_id = product UUID, got %v", inv.lastAdjust.ReferenceID)
	}
	if res.Stock != 25 {
		t.Fatalf("expected res.Stock = 25, got %d", res.Stock)
	}
	if repo.softDeleteCalls != 0 {
		t.Fatalf("product must survive successful workflow")
	}
}

// TestCreateProduct_StockAndOutboxFailureCompensates verifies the last-resort
// path: when stock init fails AND the retry event cannot be queued, the
// just-created product is removed so no hidden partial state survives.
func TestCreateProduct_StockAndOutboxFailureCompensates(t *testing.T) {
	productUUID := uuid.New()
	inv := &mockInventory{adjustErr: errors.New("inventory unavailable")}
	repo := &mockRepo{
		getCategoryByUUID: func(context.Context, uuid.UUID) (catalogsqlc.Category, error) {
			return catalogsqlc.Category{CategoryID: 3}, nil
		},
		getProductBySlug: func(context.Context, string) (catalogsqlc.GetProductBySlugRow, error) {
			return catalogsqlc.GetProductBySlugRow{}, pgx.ErrNoRows
		},
		createProduct: func(_ context.Context, _ catalogsqlc.CreateProductParams) (catalogsqlc.CreateProductRow, error) {
			return stockRowFixture(productUUID), nil
		},
		getProductByUUID: func(context.Context, uuid.UUID) (catalogsqlc.GetProductByUUIDRow, error) {
			return catalogsqlc.GetProductByUUIDRow{}, pgx.ErrNoRows
		},
		insertOutboxEvent: func(_ context.Context, _, _, eventType string, _ any) (catalogsqlc.OutboxEvent, error) {
			if eventType == "INITIAL_STOCK_SET" {
				return catalogsqlc.OutboxEvent{}, errors.New("outbox table unavailable")
			}
			return catalogsqlc.OutboxEvent{}, nil
		},
		softDeleteProduct: func(_ context.Context, _ uuid.UUID) (catalogsqlc.Product, error) {
			return catalogsqlc.Product{}, nil
		},
	}

	svc := newWorkflowService(repo, inv)
	_, err := svc.CreateProduct(context.Background(), dto.CreateProductInput{
		Name:         "P",
		CategoryUUID: uuid.NewString(),
		Stock:        25,
	})
	if err == nil {
		t.Fatal("expected error when stock init fails")
	}
	if repo.softDeleteCalls != 1 {
		t.Fatalf("expected compensation delete, got %d", repo.softDeleteCalls)
	}
}

// TestProcessOutboxBatch_RetriesInitialStock verifies the outbox worker:
// a failing stock init leaves the event PENDING, and once inventory recovers
// the event is applied and marked processed.
func TestProcessOutboxBatch_RetriesInitialStock(t *testing.T) {
	productUUID := uuid.New()
	const eventID = int64(501)
	payload, _ := json.Marshal(map[string]any{
		"product_uuid": productUUID.String(),
		"product_id":   int32(7),
		"quantity":     int32(25),
	})
	pending := []catalogsqlc.OutboxEvent{{
		EventID:       eventID,
		AggregateType: "product",
		AggregateID:   productUUID.String(),
		EventType:     "INITIAL_STOCK_SET",
		Payload:       payload,
	}}

	inv := &mockInventory{adjustErr: errors.New("inventory unavailable")}
	repo := &mockRepo{
		getPendingOutbox: func(context.Context, int32) ([]catalogsqlc.OutboxEvent, error) {
			return pending, nil
		},
	}
	svc := newWorkflowService(repo, inv)

	// Tick 1: inventory down -> event stays PENDING.
	if _, err := svc.ProcessOutboxBatch(context.Background()); err != nil {
		t.Fatalf("unexpected worker error: %v", err)
	}
	if len(repo.markedEventIDs) != 0 {
		t.Fatalf("event must stay PENDING while inventory is down, marked %v", repo.markedEventIDs)
	}

	// Tick 2: inventory back -> event applied and marked processed.
	inv.adjustErr = nil
	if _, err := svc.ProcessOutboxBatch(context.Background()); err != nil {
		t.Fatalf("unexpected worker error: %v", err)
	}
	if len(repo.markedEventIDs) != 1 || repo.markedEventIDs[0] != eventID {
		t.Fatalf("expected event %d marked processed, got %v", eventID, repo.markedEventIDs)
	}
	if inv.lastAdjust.ReferenceID == nil || *inv.lastAdjust.ReferenceID != productUUID.String() {
		t.Fatalf("expected idempotency reference = product UUID, got %v", inv.lastAdjust.ReferenceID)
	}
	if inv.lastAdjust.QuantityChange != 25 {
		t.Fatalf("expected quantity 25, got %d", inv.lastAdjust.QuantityChange)
	}
}

// TestCreateProduct_StockFailureQueuesOutbox verifies the real saga: a failed
// stock init keeps the product, enqueues an INITIAL_STOCK_SET outbox event,
// and reports a non-success error so no partial state hides.
func TestCreateProduct_StockFailureQueuesOutbox(t *testing.T) {
	productUUID := uuid.New()
	inv := &mockInventory{adjustErr: errors.New("inventory unavailable")}
	var outboxEventType string
	var outboxPayload []byte
	repo := &mockRepo{
		getCategoryByUUID: func(context.Context, uuid.UUID) (catalogsqlc.Category, error) {
			return catalogsqlc.Category{CategoryID: 3}, nil
		},
		getProductBySlug: func(context.Context, string) (catalogsqlc.GetProductBySlugRow, error) {
			return catalogsqlc.GetProductBySlugRow{}, pgx.ErrNoRows
		},
		createProduct: func(_ context.Context, _ catalogsqlc.CreateProductParams) (catalogsqlc.CreateProductRow, error) {
			return stockRowFixture(productUUID), nil
		},
		getProductByUUID: func(context.Context, uuid.UUID) (catalogsqlc.GetProductByUUIDRow, error) {
			return catalogsqlc.GetProductByUUIDRow{}, pgx.ErrNoRows
		},
		insertOutboxEvent: func(_ context.Context, _, aggregateID, eventType string, payload any) (catalogsqlc.OutboxEvent, error) {
			outboxEventType = eventType
			outboxPayload, _ = json.Marshal(payload)
			return catalogsqlc.OutboxEvent{}, nil
		},
		softDeleteProduct: func(_ context.Context, _ uuid.UUID) (catalogsqlc.Product, error) {
			return catalogsqlc.Product{}, nil
		},
	}

	svc := newWorkflowService(repo, inv)
	_, err := svc.CreateProduct(context.Background(), dto.CreateProductInput{
		Name:         "P",
		CategoryUUID: uuid.NewString(),
		Stock:        25,
	})
	if err == nil {
		t.Fatal("expected non-success error when stock init fails")
	}
	if repo.softDeleteCalls != 0 {
		t.Fatalf("product must survive; compensation delete ran %d times", repo.softDeleteCalls)
	}
	if outboxEventType != "INITIAL_STOCK_SET" {
		t.Fatalf("expected INITIAL_STOCK_SET outbox event, got %q", outboxEventType)
	}
	var payload map[string]any
	if jsonErr := json.Unmarshal(outboxPayload, &payload); jsonErr != nil {
		t.Fatalf("payload not JSON: %v", jsonErr)
	}
	if payload["product_uuid"] != productUUID.String() || payload["quantity"] != float64(25) {
		t.Fatalf("unexpected payload: %s", outboxPayload)
	}
}

// TestCreateProduct_StockInitRetryIsIdempotent verifies the retry in the
// outbox path passes the product UUID as idempotency reference (worker side).
func TestCreateProduct_StockInitRetryIsIdempotent(t *testing.T) {
	productUUID := uuid.New()
	inv := &mockInventory{}
	repo := &mockRepo{
		getCategoryByUUID: func(context.Context, uuid.UUID) (catalogsqlc.Category, error) {
			return catalogsqlc.Category{CategoryID: 3}, nil
		},
		getProductBySlug: func(context.Context, string) (catalogsqlc.GetProductBySlugRow, error) {
			return catalogsqlc.GetProductBySlugRow{}, pgx.ErrNoRows
		},
		createProduct: func(_ context.Context, _ catalogsqlc.CreateProductParams) (catalogsqlc.CreateProductRow, error) {
			return stockRowFixture(productUUID), nil
		},
		getProductByUUID: func(context.Context, uuid.UUID) (catalogsqlc.GetProductByUUIDRow, error) {
			return catalogsqlc.GetProductByUUIDRow{}, pgx.ErrNoRows
		},
		insertOutboxEvent: func(_ context.Context, _, _, _ string, _ any) (catalogsqlc.OutboxEvent, error) {
			return catalogsqlc.OutboxEvent{}, nil
		},
		softDeleteProduct: func(_ context.Context, _ uuid.UUID) (catalogsqlc.Product, error) {
			return catalogsqlc.Product{}, nil
		},
	}

	svc := newWorkflowService(repo, inv)
	// First call fails (simulating outage), second succeeds (recovery).
	inv.adjustErr = errors.New("inventory unavailable")
	_, _ = svc.CreateProduct(context.Background(), dto.CreateProductInput{
		Name: "P", CategoryUUID: uuid.NewString(), Stock: 25,
	})
	inv.adjustErr = nil
	err := svc.initStock(context.Background(), productUUID.String(), 7, 25)
	if err != nil {
		t.Fatalf("expected stock init success, got %v", err)
	}
	if inv.lastAdjust.ReferenceID == nil || *inv.lastAdjust.ReferenceID != productUUID.String() {
		t.Fatalf("expected idempotency reference = product UUID, got %v", inv.lastAdjust.ReferenceID)
	}
}

// TestCreateProduct_NoInventoryClientQueuesOutbox verifies a missing inventory
// client is treated as a transient outage: product kept, retry event queued,
// caller still gets a non-success error.
func TestCreateProduct_NoInventoryClientQueuesOutbox(t *testing.T) {
	productUUID := uuid.New()
	var outboxEventType string
	repo := &mockRepo{
		getCategoryByUUID: func(context.Context, uuid.UUID) (catalogsqlc.Category, error) {
			return catalogsqlc.Category{CategoryID: 3}, nil
		},
		getProductBySlug: func(context.Context, string) (catalogsqlc.GetProductBySlugRow, error) {
			return catalogsqlc.GetProductBySlugRow{}, pgx.ErrNoRows
		},
		createProduct: func(_ context.Context, _ catalogsqlc.CreateProductParams) (catalogsqlc.CreateProductRow, error) {
			return stockRowFixture(productUUID), nil
		},
		getProductByUUID: func(context.Context, uuid.UUID) (catalogsqlc.GetProductByUUIDRow, error) {
			return catalogsqlc.GetProductByUUIDRow{}, pgx.ErrNoRows
		},
		insertOutboxEvent: func(_ context.Context, _, _, eventType string, _ any) (catalogsqlc.OutboxEvent, error) {
			outboxEventType = eventType
			return catalogsqlc.OutboxEvent{}, nil
		},
		softDeleteProduct: func(_ context.Context, _ uuid.UUID) (catalogsqlc.Product, error) {
			return catalogsqlc.Product{}, nil
		},
	}

	svc := newWorkflowService(repo, nil)
	_, err := svc.CreateProduct(context.Background(), dto.CreateProductInput{
		Name:         "P",
		CategoryUUID: uuid.NewString(),
		Stock:        25,
	})
	if err == nil {
		t.Fatal("expected error when inventory client is missing")
	}
	if outboxEventType != "INITIAL_STOCK_SET" {
		t.Fatalf("expected INITIAL_STOCK_SET queued, got %q", outboxEventType)
	}
	if repo.softDeleteCalls != 0 {
		t.Fatalf("product must survive a transient outage, deleted %d", repo.softDeleteCalls)
	}
}
