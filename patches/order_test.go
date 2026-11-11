package model

import "testing"

func TestEpayOrderCanReselectPayment(t *testing.T) {
	order := Order{Status: OrderStatusWaiting, ApiType: OrderApiTypeEpayOrder}
	if !order.CanReselectPayment() {
		t.Fatal("EasyPay multi-currency order should allow payment method selection")
	}

	order.ApiType = OrderApiTypeEpay
	if order.CanReselectPayment() {
		t.Fatal("fixed EasyPay transaction should not allow payment method selection")
	}
}
