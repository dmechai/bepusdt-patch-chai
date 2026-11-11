package epay

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/v03413/bepusdt/app/model"
	"gorm.io/gorm"
)

func setupQueryTest(t *testing.T) (*gin.Engine, func()) {
	t.Helper()
	previousDB := model.Db
	db, err := gorm.Open(sqlite.Open(t.TempDir()+"/query.db"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	if err := db.AutoMigrate(&model.Conf{}, &model.Order{}); err != nil {
		t.Fatalf("migrate test db: %v", err)
	}
	model.Db = db
	if err := db.Create(&model.Conf{K: model.ApiAuthToken, V: "test-token"}).Error; err != nil {
		t.Fatalf("seed token: %v", err)
	}

	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/api.php", new(Epay).QueryOrder)
	router.POST("/api.php", new(Epay).QueryOrder)
	return router, func() { model.Db = previousDB }
}

func TestQueryOrderReturnsPaidEasyPayOrder(t *testing.T) {
	router, cleanup := setupQueryTest(t)
	defer cleanup()

	nowTime := time.Now()
	now := model.Datetime(nowTime)
	order := model.Order{
		OrderId:     "merchant-123",
		TradeId:     "trade-456",
		Money:       "12.50",
		ApiType:     model.OrderApiTypeEpay,
		Status:      model.OrderStatusSuccess,
		ExpiredAt:   time.Now().Add(time.Hour),
		ConfirmedAt: &nowTime,
		AutoTimeAt: model.AutoTimeAt{
			CreatedAt: &now,
			UpdatedAt: &now,
		},
	}
	if err := model.Db.Create(&order).Error; err != nil {
		t.Fatalf("seed order: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api.php?act=order&pid=1000&key=test-token&out_trade_no=merchant-123", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.Code)
	}
	for _, expected := range []string{`"code":1`, `"out_trade_no":"merchant-123"`, `"trade_no":"trade-456"`, `"money":"12.50"`, `"trade_status":"TRADE_SUCCESS"`, `"status":1`} {
		if !strings.Contains(resp.Body.String(), expected) {
			t.Errorf("response %s missing %s", resp.Body.String(), expected)
		}
	}
}

func TestQueryOrderRejectsInvalidToken(t *testing.T) {
	router, cleanup := setupQueryTest(t)
	defer cleanup()

	req := httptest.NewRequest(http.MethodGet, "/api.php?act=order&pid=1000&key=wrong&out_trade_no=merchant-123", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK || !strings.Contains(resp.Body.String(), `"code":0`) {
		t.Fatalf("unexpected response: status=%d body=%s", resp.Code, resp.Body.String())
	}
}
