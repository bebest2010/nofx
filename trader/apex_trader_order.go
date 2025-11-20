package trader

import (
	"context"
	"fmt"
	"net/http"
)

func (s *ApexClientRequest) CreateOrder(ctx context.Context, opts ...RequestOption) (res *ServerResponse, err error) {
	r := &request{
		method:   http.MethodPost,
		endpoint: "/api/v3/order",
		secType:  secTypeSigned,
	}
	data, err := SendRequest(ctx, opts, r, s, err)
	return GetServerResponse(err, data)
}

func (s *ApexClientRequest) GetOpenOrders(symbol string, ctx context.Context, opts ...RequestOption) (res *ServerResponse, err error) {
	endPoint := "/api/v3/open-orders"
	if symbol != "" {
		endPoint = fmt.Sprintf("/api/v3/open-orders?symbol=%v", symbol)
	}
	r := &request{
		method:   http.MethodGet,
		endpoint: endPoint,
		secType:  secTypeSigned,
	}
	data, err := SendRequest(ctx, opts, r, s, err)
	return GetServerResponse(err, data)
}

func (s *ApexClientRequest) CancelOrderById(ctx context.Context, opts ...RequestOption) (res *ServerResponse, err error) {
	endPoint := "/api/v3/delete-order"

	r := &request{
		method:   http.MethodPost,
		endpoint: endPoint,
		secType:  secTypeSigned,
	}
	data, err := SendRequest(ctx, opts, r, s, err)
	return GetServerResponse(err, data)
}

func (s *ApexClientRequest) CancelAllOrders(ctx context.Context, opts ...RequestOption) (res *ServerResponse, err error) {
	endPoint := "/api/v3/delete-open-orders"

	r := &request{
		method:   http.MethodPost,
		endpoint: endPoint,
		secType:  secTypeSigned,
	}
	data, err := SendRequest(ctx, opts, r, s, err)
	return GetServerResponse(err, data)
}
