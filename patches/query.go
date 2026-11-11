package epay

import (
	"crypto/subtle"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/v03413/bepusdt/app/model"
)

type orderQueryResponse struct {
	Code        int    `json:"code"`
	Msg         string `json:"msg"`
	Pid         string `json:"pid,omitempty"`
	OutTradeNo  string `json:"out_trade_no,omitempty"`
	TradeNo     string `json:"trade_no,omitempty"`
	Money       string `json:"money,omitempty"`
	TradeStatus string `json:"trade_status,omitempty"`
	Status      int    `json:"status,omitempty"`
}

// QueryOrder implements the EasyPay-compatible api.php?act=order endpoint
// used by gateways to verify a payment notification against BEpusdt.
func (Epay) QueryOrder(ctx *gin.Context) {
	if err := ctx.Request.ParseForm(); err != nil {
		ctx.JSON(http.StatusBadRequest, orderQueryResponse{Code: 0, Msg: "invalid parameters"})
		return
	}

	form := ctx.Request.Form
	if form.Get("act") != "order" || form.Get("pid") != Pid || !validQueryKey(form.Get("key"), model.AuthToken()) {
		ctx.JSON(http.StatusOK, orderQueryResponse{Code: 0, Msg: "invalid credentials or action"})
		return
	}

	outTradeNo := form.Get("out_trade_no")
	if outTradeNo == "" {
		ctx.JSON(http.StatusOK, orderQueryResponse{Code: 0, Msg: "out_trade_no is required"})
		return
	}

	var order model.Order
	result := model.Db.Where("order_id = ? AND api_type IN ?", outTradeNo, []string{
		model.OrderApiTypeEpay,
		model.OrderApiTypeEpayOrder,
	}).Order("id DESC").Limit(1).Find(&order)
	if result.Error != nil {
		ctx.JSON(http.StatusInternalServerError, orderQueryResponse{Code: 0, Msg: "order lookup failed"})
		return
	}
	if result.RowsAffected == 0 {
		ctx.JSON(http.StatusOK, orderQueryResponse{Code: 0, Msg: "order not found"})
		return
	}

	response := orderQueryResponse{
		Code:       1,
		Msg:        "success",
		Pid:        Pid,
		OutTradeNo: order.OrderId,
		TradeNo:    order.TradeId,
		Money:      order.Money,
	}
	if order.Status == model.OrderStatusSuccess {
		response.Status = 1
		response.TradeStatus = "TRADE_SUCCESS"
	}
	ctx.JSON(http.StatusOK, response)
}

func validQueryKey(provided, expected string) bool {
	if provided == "" || expected == "" || len(provided) != len(expected) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(provided), []byte(expected)) == 1
}
