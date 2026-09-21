package repository_test

import (
	"context"
	"database/sql"
	"errors"
	"sync"
	"testing"
	"time"

	"kitchen-api/internal/domain"
	"kitchen-api/internal/repository"
)

// Create orders and check transaction
func TestCreateOrder_Success(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()

	restID := createTestRestaurant(t, "Burger Place", "key_burger", true)
	item1ID := createTestMenuItem(t, restID, "burger_classic", "Бургер Классический", 45000, true)
	item2ID := createTestMenuItem(t, restID, "fries", "Картофель фри", 18000, true)

	order := domain.Order{
		UserID:          "user_100",
		RestaurantID:    restID,
		Status:          domain.StatusCreated,
		TotalPriceCents: 45000 + 2*18000,
		Items: []domain.OrderItem{
			{MenuItemID: item1ID, Quantity: 1, UnitPriceCents: 45000},
			{MenuItemID: item2ID, Quantity: 2, UnitPriceCents: 18000},
		},
	}

	err := testRepo.CreateOrder(ctx, &order)
	if err != nil {
		t.Fatalf("unexpected error creating order: %v", err)
	}

	if order.ID <= 0 {
		t.Fatalf("expected order ID > 0, got %d", order.ID)
	}
	if order.CreatedAt.IsZero() || order.UpdatedAt.IsZero() {
		t.Fatal("expected order timestamps to be set")
	}
	if len(order.Items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(order.Items))
	}
	for i, item := range order.Items {
		if item.ID <= 0 {
			t.Fatalf("expected item[%d].ID > 0, got %d", i, item.ID)
		}
		if item.OrderID != order.ID {
			t.Fatalf("expected item[%d].OrderID == %d, got %d", i, order.ID, item.OrderID)
		}
		if item.CreatedAt.IsZero() {
			t.Fatalf("expected item[%d].CreatedAt not to be zero", i)
		}
	}

	// Direct DB checks
	var dbStatus string
	var dbTotal int64
	var dbUserID string
	err = testPool.QueryRow(ctx, "SELECT status, total_price_cents, user_id FROM orders WHERE id = $1", order.ID).
		Scan(&dbStatus, &dbTotal, &dbUserID)
	if err != nil {
		t.Fatalf("failed to query created order: %v", err)
	}
	if dbStatus != string(domain.StatusCreated) {
		t.Errorf("expected status %s, got %s", domain.StatusCreated, dbStatus)
	}
	if dbTotal != 81000 {
		t.Errorf("expected total 81000, got %d", dbTotal)
	}
	if dbUserID != "user_100" {
		t.Errorf("expected user_id user_100, got %s", dbUserID)
	}

	var itemsCount int
	err = testPool.QueryRow(ctx, "SELECT count(*) FROM order_items WHERE order_id = $1", order.ID).Scan(&itemsCount)
	if err != nil {
		t.Fatalf("failed to query order items count: %v", err)
	}
	if itemsCount != 2 {
		t.Errorf("expected 2 order items in DB, got %d", itemsCount)
	}

	// Audit log check: initial event must be (old_status NULL, new_status created)
	var oldStatus sql.NullString
	var newStatus string
	err = testPool.QueryRow(ctx, "SELECT old_status, new_status FROM order_events WHERE order_id = $1", order.ID).
		Scan(&oldStatus, &newStatus)
	if err != nil {
		t.Fatalf("failed to query order event: %v", err)
	}
	if oldStatus.Valid {
		t.Errorf("expected initial old_status to be NULL, got %s", oldStatus.String)
	}
	if newStatus != string(domain.StatusCreated) {
		t.Errorf("expected initial new_status to be %s, got %s", domain.StatusCreated, newStatus)
	}
}

func TestCreateOrder_RollbackOnFKError(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()

	restID := createTestRestaurant(t, "Pizza House", "key_pizza", true)

	order := domain.Order{
		UserID:          "user_200",
		RestaurantID:    restID,
		Status:          domain.StatusCreated,
		TotalPriceCents: 5000,
		Items: []domain.OrderItem{
			{MenuItemID: 999999, Quantity: 1, UnitPriceCents: 5000}, // non-existent menu item
		},
	}

	err := testRepo.CreateOrder(ctx, &order)
	if err == nil {
		t.Fatal("expected error due to foreign key violation, got nil")
	}

	// Check that transaction was rolled back
	var ordersCount, itemsCount, eventsCount int
	_ = testPool.QueryRow(ctx, "SELECT count(*) FROM orders").Scan(&ordersCount)
	_ = testPool.QueryRow(ctx, "SELECT count(*) FROM order_items").Scan(&itemsCount)
	_ = testPool.QueryRow(ctx, "SELECT count(*) FROM order_events").Scan(&eventsCount)

	if ordersCount != 0 {
		t.Errorf("expected 0 orders due to rollback, got %d", ordersCount)
	}
	if itemsCount != 0 {
		t.Errorf("expected 0 order_items due to rollback, got %d", itemsCount)
	}
	if eventsCount != 0 {
		t.Errorf("expected 0 order_events due to rollback, got %d", eventsCount)
	}
}

func TestCreateOrder_ServerPriceCalculationViaDomainService(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()

	restID := createTestRestaurant(t, "Taco Shop", "key_taco", true)
	item1ID := createTestMenuItem(t, restID, "taco_beef", "Beef Taco", 30000, true)
	item2ID := createTestMenuItem(t, restID, "nachos", "Nachos", 25000, true)
	itemUnavailableID := createTestMenuItem(t, restID, "soda", "Soda", 10000, false)

	orderService := domain.NewOrderService(testRepo)

	// Valid order: price must be calculated by service from DB
	order, err := orderService.CreateOrder(ctx, "user_taco_lover", restID, []domain.OrderItemCreate{
		{MenuItemID: item1ID, Quantity: 2},
		{MenuItemID: item2ID, Quantity: 3},
	})
	if err != nil {
		t.Fatalf("unexpected error creating order: %v", err)
	}

	expectedPrice := int64(2*30000 + 3*25000)
	if order.TotalPriceCents != expectedPrice {
		t.Errorf("expected total price %d, got %d", expectedPrice, order.TotalPriceCents)
	}

	// Ordering unavailable item must fail
	_, err = orderService.CreateOrder(ctx, "user_taco_lover", restID, []domain.OrderItemCreate{
		{MenuItemID: itemUnavailableID, Quantity: 1},
	})
	if !errors.Is(err, domain.ErrMenuItemUnavailable) {
		t.Errorf("expected ErrMenuItemUnavailable, got %v", err)
	}

	// Ordering from inactive restaurant must fail
	inactiveRestID := createTestRestaurant(t, "Closed Shop", "key_closed", false)
	createTestMenuItem(t, inactiveRestID, "dish", "Dish", 10000, true)
	_, err = orderService.CreateOrder(ctx, "user_taco_lover", inactiveRestID, []domain.OrderItemCreate{
		{MenuItemID: item1ID, Quantity: 1},
	})
	if !errors.Is(err, domain.ErrRestaurantUnavailable) {
		t.Errorf("expected ErrRestaurantUnavailable, got %v", err)
	}
}

func TestOrder_TimestampTrigger(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()

	restID := createTestRestaurant(t, "Sushi Bar", "key_sushi", true)
	itemID := createTestMenuItem(t, restID, "sushi_set", "Sushi Set", 120000, true)

	order := domain.Order{
		UserID:          "user_sushi",
		RestaurantID:    restID,
		Status:          domain.StatusCreated,
		TotalPriceCents: 120000,
		Items:           []domain.OrderItem{{MenuItemID: itemID, Quantity: 1, UnitPriceCents: 120000}},
	}
	err := testRepo.CreateOrder(ctx, &order)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	initialUpdatedAt := order.UpdatedAt

	// Small pause so that now() in PostgreSQL will be strictly greater
	time.Sleep(20 * time.Millisecond)

	_, err = testPool.Exec(ctx, "UPDATE orders SET total_price_cents = 130000 WHERE id = $1", order.ID)
	if err != nil {
		t.Fatalf("failed to update order: %v", err)
	}

	var newUpdatedAt time.Time
	err = testPool.QueryRow(ctx, "SELECT updated_at FROM orders WHERE id = $1", order.ID).Scan(&newUpdatedAt)
	if err != nil {
		t.Fatalf("failed to query updated_at: %v", err)
	}

	if !newUpdatedAt.After(initialUpdatedAt) {
		t.Errorf("expected updated_at (%v) to be after initial (%v)", newUpdatedAt, initialUpdatedAt)
	}
}

// intercept CreateOrder func to simulate restaurant update item status, while user creating order
type interceptedRepo struct {
	*repository.Repo
	beforeCreateOrder func()
}

func (r *interceptedRepo) CreateOrder(ctx context.Context, order *domain.Order) error {
	if r.beforeCreateOrder != nil {
		r.beforeCreateOrder()
	}
	return r.Repo.CreateOrder(ctx, order)
}

func TestCreateOrder_ItemRace(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()

	restID := createTestRestaurant(t, "Diner", "key_diner", true)
	itemID := createTestMenuItem(t, restID, "limited_steak", "limited steak", 80000, true)

	intercepted := &interceptedRepo{
		Repo: testRepo,
	}

	// after domain.OrderService, but before inserting onto DB
	intercepted.beforeCreateOrder = func() {
		err := testRepo.UpsertMenuItems(ctx, restID, []domain.MenuItem{
			{
				ExternalID:  "limited_steak",
				Name:        "limited steak",
				PriceCents:  80000,
				IsAvailable: false,
			},
		})
		if err != nil {
			t.Fatalf("failed to update menu availability: %v", err)
		}
	}

	orderService := domain.NewOrderService(intercepted)

	order, err := orderService.CreateOrder(ctx, "user_toctou", restID, []domain.OrderItemCreate{
		{MenuItemID: itemID, Quantity: 1},
	})

	// if race is prevented -> no orders created in DB
	if err == nil {
		t.Fatalf("Error: order %d was successfully placed, but is not available", order.ID)
	}
	if !errors.Is(err, domain.ErrMenuItemUnavailable) {
		t.Fatalf("expected ErrMenuItemUnavailable, got %v", err)
	}

	// 0 orders should be created in DB
	var ordersCount int
	_ = testPool.QueryRow(ctx, "SELECT count(*) FROM orders WHERE restaurant_id = $1", restID).Scan(&ordersCount)
	if ordersCount != 0 {
		t.Fatalf("expected 0 orders in DB, found %d", ordersCount)
	}
}

// Order lifecycle and order_events
func TestUpdateOrderStatus_ValidTransitionsAndAudit(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()

	restID := createTestRestaurant(t, "Burger Lab", "key_lab", true)
	itemID := createTestMenuItem(t, restID, "lab_burger", "Lab Burger", 50000, true)

	order := domain.Order{
		UserID:          "user_foodie",
		RestaurantID:    restID,
		Status:          domain.StatusCreated,
		TotalPriceCents: 50000,
		Items:           []domain.OrderItem{{MenuItemID: itemID, Quantity: 1, UnitPriceCents: 50000}},
	}
	err := testRepo.CreateOrder(ctx, &order)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Sequence: created -> accepted -> cooking -> ready -> completed
	transitions := []domain.OrderStatus{
		domain.StatusAccepted,
		domain.StatusCooking,
		domain.StatusReady,
		domain.StatusCompleted,
	}

	for _, nextStatus := range transitions {
		err := testRepo.UpdateOrderStatus(ctx, order.ID, nextStatus)
		if err != nil {
			t.Fatalf("failed transition to %s: %v", nextStatus, err)
		}

		// Verify order status in DB after step
		var curStatus string
		err = testPool.QueryRow(ctx, "SELECT status FROM orders WHERE id = $1", order.ID).Scan(&curStatus)
		if err != nil {
			t.Fatalf("failed to query current status: %v", err)
		}
		if curStatus != string(nextStatus) {
			t.Fatalf("expected status %s, got %s", nextStatus, curStatus)
		}
	}

	// Verify order_events history: initial + 4 transitions = 5 records
	rows, err := testPool.Query(ctx,
		"SELECT old_status, new_status FROM order_events WHERE order_id = $1 ORDER BY id ASC",
		order.ID,
	)
	if err != nil {
		t.Fatalf("failed to query order events: %v", err)
	}
	defer rows.Close()

	type eventRow struct {
		oldStatus sql.NullString
		newStatus string
	}
	var events []eventRow
	for rows.Next() {
		var er eventRow
		if err := rows.Scan(&er.oldStatus, &er.newStatus); err != nil {
			t.Fatalf("failed to scan event row: %v", err)
		}
		events = append(events, er)
	}

	if len(events) != 5 {
		t.Fatalf("expected 5 audit events, got %d", len(events))
	}

	expectedEvents := []struct {
		hasOld    bool
		oldStatus string
		newStatus string
	}{
		{false, "", string(domain.StatusCreated)},
		{true, string(domain.StatusCreated), string(domain.StatusAccepted)},
		{true, string(domain.StatusAccepted), string(domain.StatusCooking)},
		{true, string(domain.StatusCooking), string(domain.StatusReady)},
		{true, string(domain.StatusReady), string(domain.StatusCompleted)},
	}

	for i, exp := range expectedEvents {
		if exp.hasOld {
			if !events[i].oldStatus.Valid || events[i].oldStatus.String != exp.oldStatus {
				t.Errorf("event[%d]: expected old_status %s, got %v", i, exp.oldStatus, events[i].oldStatus)
			}
		} else {
			if events[i].oldStatus.Valid {
				t.Errorf("event[%d]: expected old_status NULL, got %s", i, events[i].oldStatus.String)
			}
		}
		if events[i].newStatus != exp.newStatus {
			t.Errorf("event[%d]: expected new_status %s, got %s", i, exp.newStatus, events[i].newStatus)
		}
	}
}

func TestUpdateOrderStatus_InvalidTransitions(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()

	restID := createTestRestaurant(t, "Pasta Bar", "key_pasta", true)
	itemID := createTestMenuItem(t, restID, "carbonara", "Carbonara", 60000, true)

	t.Run("cooking cannot transition to cancelled", func(t *testing.T) {
		order := domain.Order{
			UserID:          "user_p1",
			RestaurantID:    restID,
			Status:          domain.StatusCreated,
			TotalPriceCents: 60000,
			Items:           []domain.OrderItem{{MenuItemID: itemID, Quantity: 1, UnitPriceCents: 60000}},
		}
		if err := testRepo.CreateOrder(ctx, &order); err != nil {
			t.Fatalf("failed to create order: %v", err)
		}

		if err := testRepo.UpdateOrderStatus(ctx, order.ID, domain.StatusAccepted); err != nil {
			t.Fatalf("failed to accept order: %v", err)
		}
		if err := testRepo.UpdateOrderStatus(ctx, order.ID, domain.StatusCooking); err != nil {
			t.Fatalf("failed to start cooking: %v", err)
		}

		err := testRepo.UpdateOrderStatus(ctx, order.ID, domain.StatusCancelled)
		if !errors.Is(err, domain.ErrInvalidStatusTransition) {
			t.Errorf("expected ErrInvalidStatusTransition, got %v", err)
		}

		// Verify status remains cooking in DB
		var status string
		_ = testPool.QueryRow(ctx, "SELECT status FROM orders WHERE id = $1", order.ID).Scan(&status)
		if status != string(domain.StatusCooking) {
			t.Errorf("expected status to remain cooking, got %s", status)
		}
	})

	t.Run("cancelled order cannot be cancelled again or transitioned", func(t *testing.T) {
		order := domain.Order{
			UserID:          "user_p2",
			RestaurantID:    restID,
			Status:          domain.StatusCreated,
			TotalPriceCents: 60000,
			Items:           []domain.OrderItem{{MenuItemID: itemID, Quantity: 1, UnitPriceCents: 60000}},
		}
		if err := testRepo.CreateOrder(ctx, &order); err != nil {
			t.Fatalf("failed to create order: %v", err)
		}

		// First cancellation allowed
		if err := testRepo.UpdateOrderStatus(ctx, order.ID, domain.StatusCancelled); err != nil {
			t.Fatalf("expected cancellation to succeed, got %v", err)
		}

		// Cancelling already cancelled order via OrderService = ErrOrderAlreadyCancelled
		orderService := domain.NewOrderService(testRepo)
		err := orderService.CancelOrder(ctx, order.ID, "user_p2")
		if !errors.Is(err, domain.ErrOrderAlreadyCancelled) {
			t.Errorf("expected ErrOrderAlreadyCancelled from orderService.CancelOrder, got %v", err)
		}

		// Transition from cancelled to accepted via repo = ErrOrderAlreadyCancelled
		err = testRepo.UpdateOrderStatus(ctx, order.ID, domain.StatusAccepted)
		if !errors.Is(err, domain.ErrOrderAlreadyCancelled) {
			t.Errorf("expected ErrOrderAlreadyCancelled from repo, got %v", err)
		}
	})
}

func TestUpdateOrderStatus_SameStatusNoOp(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()

	restID := createTestRestaurant(t, "Wok House", "key_wok", true)
	itemID := createTestMenuItem(t, restID, "noodles", "Noodles", 35000, true)

	order := domain.Order{
		UserID:          "user_wok",
		RestaurantID:    restID,
		Status:          domain.StatusCreated,
		TotalPriceCents: 35000,
		Items:           []domain.OrderItem{{MenuItemID: itemID, Quantity: 1, UnitPriceCents: 35000}},
	}
	if err := testRepo.CreateOrder(ctx, &order); err != nil {
		t.Fatalf("failed to create order: %v", err)
	}

	if err := testRepo.UpdateOrderStatus(ctx, order.ID, domain.StatusAccepted); err != nil {
		t.Fatalf("failed to accept order: %v", err)
	}

	// Call with same status again
	err := testRepo.UpdateOrderStatus(ctx, order.ID, domain.StatusAccepted)
	if err != nil {
		t.Fatalf("expected nil error on same status, got %v", err)
	}

	// Must NOT insert duplicate event
	var count int
	_ = testPool.QueryRow(ctx, "SELECT count(*) FROM order_events WHERE order_id = $1", order.ID).Scan(&count)
	if count != 2 { // 1 initial + 1 for accepted
		t.Errorf("expected 2 events in total, got %d", count)
	}
}

func TestUpdateOrderStatus_OrderNotFound(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()

	err := testRepo.UpdateOrderStatus(ctx, 999999, domain.StatusAccepted)
	if !errors.Is(err, domain.ErrOrderNotFound) {
		t.Fatalf("expected ErrOrderNotFound, got %v", err)
	}
}

func TestUpdateOrderStatus_Concurrency(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()

	restID := createTestRestaurant(t, "Steak House", "key_steak", true)
	itemID := createTestMenuItem(t, restID, "ribeye", "Ribeye Steak", 150000, true)

	order := domain.Order{
		UserID:          "user_meat",
		RestaurantID:    restID,
		Status:          domain.StatusCreated,
		TotalPriceCents: 150000,
		Items:           []domain.OrderItem{{MenuItemID: itemID, Quantity: 1, UnitPriceCents: 150000}},
	}
	if err := testRepo.CreateOrder(ctx, &order); err != nil {
		t.Fatalf("failed to create order: %v", err)
	}

	// Concurrently attempt to change status: 5 try 'accepted', 5 try 'cancelled'
	const workers = 10
	var wg sync.WaitGroup
	wg.Add(workers)

	for i := 0; i < workers; i++ {
		targetStatus := domain.StatusAccepted
		if i%2 == 1 {
			targetStatus = domain.StatusCancelled
		}
		go func(status domain.OrderStatus) {
			defer wg.Done()
			_ = testRepo.UpdateOrderStatus(ctx, order.ID, status)
		}(targetStatus)
	}
	wg.Wait()

	// Final status in DB must be valid and be accepted or cancelled
	var finalStatus string
	err := testPool.QueryRow(ctx, "SELECT status FROM orders WHERE id = $1", order.ID).Scan(&finalStatus)
	if err != nil {
		t.Fatalf("failed to query final status: %v", err)
	}
	if finalStatus != string(domain.StatusAccepted) && finalStatus != string(domain.StatusCancelled) {
		t.Fatalf("unexpected final status: %s", finalStatus)
	}

	// order_events must be consistent:
	// either 2 events (created -> winner) or 3 events (created -> accepted -> cancelled)
	rows, err := testPool.Query(ctx, "SELECT old_status, new_status FROM order_events WHERE order_id = $1 ORDER BY id ASC", order.ID)
	if err != nil {
		t.Fatalf("failed to query order events: %v", err)
	}
	defer rows.Close()

	type eventRow struct {
		oldStatus sql.NullString
		newStatus string
	}
	var events []eventRow
	for rows.Next() {
		var er eventRow
		if err := rows.Scan(&er.oldStatus, &er.newStatus); err != nil {
			t.Fatalf("failed to scan event: %v", err)
		}
		events = append(events, er)
	}

	if len(events) < 2 || len(events) > 3 {
		t.Fatalf("expected 2 or 3 events, got %d", len(events))
	}

	// Initial event must be (NULL -> created)
	if events[0].oldStatus.Valid || events[0].newStatus != string(domain.StatusCreated) {
		t.Errorf("initial event invalid: %+v", events[0])
	}

	// Every subsequent transition must strictly chain from the previous state and be valid
	for i := 1; i < len(events); i++ {
		prevNew := events[i-1].newStatus
		if !events[i].oldStatus.Valid || events[i].oldStatus.String != prevNew {
			t.Errorf("broken transition chain at %d: prev new %s != curr old %v", i, prevNew, events[i].oldStatus)
		}
		if !domain.CanTransition(domain.OrderStatus(events[i].oldStatus.String), domain.OrderStatus(events[i].newStatus)) {
			t.Errorf("invalid transition at %d: %s -> %s", i, events[i].oldStatus.String, events[i].newStatus)
		}
	}

	// The last event's new_status must match final status in orders table
	lastEvent := events[len(events)-1]
	if lastEvent.newStatus != finalStatus {
		t.Errorf("last event status %s does not match final status in orders %s", lastEvent.newStatus, finalStatus)
	}
}

// Partners menu handling
func TestUpsertMenuItems_InsertAndUpdate(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()

	restID := createTestRestaurant(t, "Bakery", "key_bakery", true)

	// initial upsert of 2 items
	initialItems := []domain.MenuItem{
		{ExternalID: "croissant", Name: "Круассан", PriceCents: 15000, IsAvailable: true},
		{ExternalID: "baguette", Name: "Багет", PriceCents: 12000, IsAvailable: true},
	}
	err := testRepo.UpsertMenuItems(ctx, restID, initialItems)
	if err != nil {
		t.Fatalf("failed initial upsert: %v", err)
	}

	menu, err := testRepo.GetMenuByRestaurantID(ctx, restID)
	if err != nil {
		t.Fatalf("failed to get menu: %v", err)
	}
	if len(menu) != 2 {
		t.Fatalf("expected 2 items, got %d", len(menu))
	}

	// secondary upsert:
	// - croissant price and name changed
	// - baguette made unavailable
	// - new item "donut" added
	updatedItems := []domain.MenuItem{
		{ExternalID: "croissant", Name: "Круассан с маслом", PriceCents: 18000, IsAvailable: true},
		{ExternalID: "baguette", Name: "Багет", PriceCents: 12000, IsAvailable: false},
		{ExternalID: "donut", Name: "Пончик", PriceCents: 10000, IsAvailable: true},
	}
	err = testRepo.UpsertMenuItems(ctx, restID, updatedItems)
	if err != nil {
		t.Fatalf("failed secondary upsert: %v", err)
	}

	// Must be exactly 3 items in total
	menu, err = testRepo.GetMenuByRestaurantID(ctx, restID)
	if err != nil {
		t.Fatalf("failed to get menu: %v", err)
	}
	if len(menu) != 3 {
		t.Fatalf("expected 3 items, got %d", len(menu))
	}

	menuMap := make(map[string]domain.MenuItem)
	for _, m := range menu {
		menuMap[m.ExternalID] = m
	}

	croissant, ok := menuMap["croissant"]
	if !ok || croissant.Name != "Круассан с маслом" || croissant.PriceCents != 18000 || !croissant.IsAvailable {
		t.Errorf("croissant was not updated properly: %+v", croissant)
	}

	baguette, ok := menuMap["baguette"]
	if !ok || baguette.IsAvailable != false {
		t.Errorf("baguette should be unavailable: %+v", baguette)
	}

	donut, ok := menuMap["donut"]
	if !ok || donut.PriceCents != 10000 || !donut.IsAvailable {
		t.Errorf("donut was not created properly: %+v", donut)
	}
}

func TestGetActiveMenuByRestaurantID(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()

	restID := createTestRestaurant(t, "Coffee Point", "key_coffee", true)
	createTestMenuItem(t, restID, "espresso", "Эспрессо", 12000, true)
	createTestMenuItem(t, restID, "latte", "Латте (нет молока)", 18000, false)

	// only espresso
	activeMenu, err := testRepo.GetActiveMenuByRestaurantID(ctx, restID)
	if err != nil {
		t.Fatalf("failed to get active menu: %v", err)
	}
	if len(activeMenu) != 1 {
		t.Fatalf("expected 1 active item, got %d", len(activeMenu))
	}
	if activeMenu[0].ExternalID != "espresso" {
		t.Errorf("expected espresso, got %s", activeMenu[0].ExternalID)
	}

	// both items
	fullMenu, err := testRepo.GetMenuByRestaurantID(ctx, restID)
	if err != nil {
		t.Fatalf("failed to get full menu: %v", err)
	}
	if len(fullMenu) != 2 {
		t.Fatalf("expected 2 items in full menu, got %d", len(fullMenu))
	}
}

func TestMenu_RestaurantIsolation(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()

	rest1 := createTestRestaurant(t, "Diner 1", "key_d1", true)
	rest2 := createTestRestaurant(t, "Diner 2", "key_d2", true)

	// Both restaurants have the same external_id "special"
	err := testRepo.UpsertMenuItems(ctx, rest1, []domain.MenuItem{
		{ExternalID: "special", Name: "Diner 1 Special", PriceCents: 50000, IsAvailable: true},
	})
	if err != nil {
		t.Fatalf("failed upsert for rest1: %v", err)
	}

	err = testRepo.UpsertMenuItems(ctx, rest2, []domain.MenuItem{
		{ExternalID: "special", Name: "Diner 2 Special", PriceCents: 80000, IsAvailable: true},
	})
	if err != nil {
		t.Fatalf("failed upsert for rest2: %v", err)
	}

	menu1, _ := testRepo.GetMenuByRestaurantID(ctx, rest1)
	menu2, _ := testRepo.GetMenuByRestaurantID(ctx, rest2)

	if len(menu1) != 1 || menu1[0].PriceCents != 50000 {
		t.Errorf("rest1 menu incorrect: %+v", menu1)
	}
	if len(menu2) != 1 || menu2[0].PriceCents != 80000 {
		t.Errorf("rest2 menu incorrect: %+v", menu2)
	}
}

// Select orders
func TestGetOrdersByUserID_WithItems(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()

	restID := createTestRestaurant(t, "Pizzeria", "key_pizz", true)
	item1ID := createTestMenuItem(t, restID, "margherita", "Маргарита", 45000, true)
	item2ID := createTestMenuItem(t, restID, "cola", "Кола", 12000, true)

	// Order 1 for user_bob
	order1 := domain.Order{
		UserID:          "user_bob",
		RestaurantID:    restID,
		Status:          domain.StatusCreated,
		TotalPriceCents: 45000,
		Items:           []domain.OrderItem{{MenuItemID: item1ID, Quantity: 1, UnitPriceCents: 45000}},
	}
	if err := testRepo.CreateOrder(ctx, &order1); err != nil {
		t.Fatalf("failed to create order1: %v", err)
	}

	time.Sleep(15 * time.Millisecond) // ensure created_at difference

	// Order 2 for user_bob (with 2 items)
	order2 := domain.Order{
		UserID:          "user_bob",
		RestaurantID:    restID,
		Status:          domain.StatusAccepted,
		TotalPriceCents: 45000 + 2*12000,
		Items: []domain.OrderItem{
			{MenuItemID: item1ID, Quantity: 1, UnitPriceCents: 45000},
			{MenuItemID: item2ID, Quantity: 2, UnitPriceCents: 12000},
		},
	}
	if err := testRepo.CreateOrder(ctx, &order2); err != nil {
		t.Fatalf("failed to create order2: %v", err)
	}

	// Order 3 for user_alice (should not appear in user_bob's orders)
	order3 := domain.Order{
		UserID:          "user_alice",
		RestaurantID:    restID,
		Status:          domain.StatusCreated,
		TotalPriceCents: 12000,
		Items:           []domain.OrderItem{{MenuItemID: item2ID, Quantity: 1, UnitPriceCents: 12000}},
	}
	if err := testRepo.CreateOrder(ctx, &order3); err != nil {
		t.Fatalf("failed to create order3: %v", err)
	}

	orders, err := testRepo.GetOrdersByUserID(ctx, "user_bob")
	if err != nil {
		t.Fatalf("failed to get orders for user_bob: %v", err)
	}

	if len(orders) != 2 {
		t.Fatalf("expected 2 orders for user_bob, got %d", len(orders))
	}

	// Verify sorting by created_at DESC: order2 must be first
	if orders[0].ID != order2.ID {
		t.Errorf("expected newest order2 (%d) first, got %d", order2.ID, orders[0].ID)
	}
	if orders[1].ID != order1.ID {
		t.Errorf("expected older order1 (%d) second, got %d", order1.ID, orders[1].ID)
	}

	// Verify items are populated for both orders
	if len(orders[0].Items) != 2 {
		t.Errorf("expected 2 items for order2, got %d", len(orders[0].Items))
	}
	if len(orders[1].Items) != 1 {
		t.Errorf("expected 1 item for order1, got %d", len(orders[1].Items))
	}

	// Empty list for unknown user
	emptyOrders, err := testRepo.GetOrdersByUserID(ctx, "unknown_user")
	if err != nil {
		t.Fatalf("expected nil error for empty orders, got %v", err)
	}
	if len(emptyOrders) != 0 {
		t.Errorf("expected 0 orders, got %d", len(emptyOrders))
	}
}

func TestGetOrdersByRestaurantAndStatus(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()

	restA := createTestRestaurant(t, "Rest A", "key_a", true)
	restB := createTestRestaurant(t, "Rest B", "key_b", true)
	itemA := createTestMenuItem(t, restA, "item_a", "Item A", 20000, true)
	itemB := createTestMenuItem(t, restB, "item_b", "Item B", 20000, true)

	// Rest A: 2 created, 1 cooking
	orderA1 := domain.Order{
		UserID: "u1", RestaurantID: restA, Status: domain.StatusCreated, TotalPriceCents: 20000,
		Items: []domain.OrderItem{{MenuItemID: itemA, Quantity: 1, UnitPriceCents: 20000}},
	}
	orderA2 := domain.Order{
		UserID: "u2", RestaurantID: restA, Status: domain.StatusCreated, TotalPriceCents: 20000,
		Items: []domain.OrderItem{{MenuItemID: itemA, Quantity: 1, UnitPriceCents: 20000}},
	}
	orderA3 := domain.Order{
		UserID: "u3", RestaurantID: restA, Status: domain.StatusCooking, TotalPriceCents: 20000,
		Items: []domain.OrderItem{{MenuItemID: itemA, Quantity: 1, UnitPriceCents: 20000}},
	}

	// Rest B: 1 created
	orderB1 := domain.Order{
		UserID: "u4", RestaurantID: restB, Status: domain.StatusCreated, TotalPriceCents: 20000,
		Items: []domain.OrderItem{{MenuItemID: itemB, Quantity: 1, UnitPriceCents: 20000}},
	}

	_ = testRepo.CreateOrder(ctx, &orderA1)
	_ = testRepo.CreateOrder(ctx, &orderA2)
	_ = testRepo.CreateOrder(ctx, &orderA3)
	_ = testRepo.CreateOrder(ctx, &orderB1)

	// 1. Filter restA with status = created
	statusCreated := domain.StatusCreated
	ordersCreated, err := testRepo.GetOrdersByRestaurantAndStatus(ctx, restA, &statusCreated)
	if err != nil {
		t.Fatalf("failed to get created orders: %v", err)
	}
	if len(ordersCreated) != 2 {
		t.Fatalf("expected 2 created orders for restA, got %d", len(ordersCreated))
	}
	for _, o := range ordersCreated {
		if o.Status != domain.StatusCreated {
			t.Errorf("expected status created, got %s", o.Status)
		}
		if o.RestaurantID != restA {
			t.Errorf("expected restaurant %d, got %d", restA, o.RestaurantID)
		}
		if len(o.Items) != 1 {
			t.Errorf("expected items to be populated, got %d", len(o.Items))
		}
	}

	// 2. Filter restA with status = nil (all orders)
	allOrdersA, err := testRepo.GetOrdersByRestaurantAndStatus(ctx, restA, nil)
	if err != nil {
		t.Fatalf("failed to get all orders for restA: %v", err)
	}
	if len(allOrdersA) != 3 {
		t.Fatalf("expected 3 total orders for restA, got %d", len(allOrdersA))
	}
}

func TestGetOrderByID(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()

	restID := createTestRestaurant(t, "Burger Hub", "key_hub", true)
	itemID := createTestMenuItem(t, restID, "hub_burger", "Hub Burger", 40000, true)

	order := domain.Order{
		UserID:          "user_hub",
		RestaurantID:    restID,
		Status:          domain.StatusCreated,
		TotalPriceCents: 40000,
		Items:           []domain.OrderItem{{MenuItemID: itemID, Quantity: 1, UnitPriceCents: 40000}},
	}
	if err := testRepo.CreateOrder(ctx, &order); err != nil {
		t.Fatalf("failed to create order: %v", err)
	}

	// Existing order
	foundOrder, err := testRepo.GetOrderByID(ctx, order.ID)
	if err != nil {
		t.Fatalf("failed to get order by id: %v", err)
	}
	if foundOrder.ID != order.ID {
		t.Errorf("expected id %d, got %d", order.ID, foundOrder.ID)
	}
	if foundOrder.UserID != "user_hub" {
		t.Errorf("expected user_hub, got %s", foundOrder.UserID)
	}
	if len(foundOrder.Items) != 1 {
		t.Errorf("expected 1 item, got %d", len(foundOrder.Items))
	}

	// Non-existing order
	_, err = testRepo.GetOrderByID(ctx, 999999)
	if !errors.Is(err, domain.ErrOrderNotFound) {
		t.Fatalf("expected ErrOrderNotFound, got %v", err)
	}
}

func TestRestaurants_Queries(t *testing.T) {
	cleanDB(t)
	ctx := context.Background()

	activeID := createTestRestaurant(t, "Active Rest", "api_key_active", true)
	_ = createTestRestaurant(t, "Inactive Rest", "api_key_inactive", false)

	// 1. GetActiveRestaurants: must only return active restaurant
	activeList, err := testRepo.GetActiveRestaurants(ctx)
	if err != nil {
		t.Fatalf("failed to get active restaurants: %v", err)
	}
	if len(activeList) != 1 {
		t.Fatalf("expected 1 active restaurant, got %d", len(activeList))
	}
	if activeList[0].ID != activeID {
		t.Errorf("expected active restaurant id %d, got %d", activeID, activeList[0].ID)
	}

	// 2. GetRestaurantByID
	rest, err := testRepo.GetRestaurantByID(ctx, activeID)
	if err != nil {
		t.Fatalf("failed to get restaurant by id: %v", err)
	}
	if rest.Name != "Active Rest" {
		t.Errorf("expected name 'Active Rest', got %s", rest.Name)
	}

	_, err = testRepo.GetRestaurantByID(ctx, 999999)
	if !errors.Is(err, domain.ErrRestaurantNotFound) {
		t.Fatalf("expected ErrRestaurantNotFound, got %v", err)
	}

	// 3. GetRestaurantByAPIKey
	byKey, err := testRepo.GetRestaurantByAPIKey(ctx, "api_key_active")
	if err != nil {
		t.Fatalf("failed to get restaurant by key: %v", err)
	}
	if byKey.ID != activeID {
		t.Errorf("expected id %d, got %d", activeID, byKey.ID)
	}

	_, err = testRepo.GetRestaurantByAPIKey(ctx, "non_existent_key")
	if !errors.Is(err, domain.ErrRestaurantNotFound) {
		t.Fatalf("expected ErrRestaurantNotFound, got %v", err)
	}
}
