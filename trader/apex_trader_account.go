package trader

import (
	"context"
	"net/http"
)

func (s *ApexClientRequest) GetAccountInfo(ctx context.Context, opts ...RequestOption) (res *ServerResponse, err error) {
	r := &request{
		method:   http.MethodGet,
		endpoint: "/api/v3/account",
		secType:  secTypeSigned,
	}
	data, err := SendRequest(ctx, opts, r, s, err)
	return GetServerResponse(err, data)
}

func (s *ApexClientRequest) GetAccountBalance(ctx context.Context, opts ...RequestOption) (res *ServerResponse, err error) {
	r := &request{
		method:   http.MethodGet,
		endpoint: "/api/v3/account-balance",
		secType:  secTypeSigned,
	}
	data, err := SendRequest(ctx, opts, r, s, err)
	return GetServerResponse(err, data)
}

func (s *ApexClientRequest) GetFundingInfo(ctx context.Context, opts ...RequestOption) (res *ServerResponse, err error) {
	r := &request{
		method:   http.MethodGet,
		endpoint: "/api/v3/funding",
		secType:  secTypeSigned,
	}
	data, err := SendRequest(ctx, opts, r, s, err)
	return GetServerResponse(err, data)
}

func (s *ApexClientRequest) GetHistoricalPnl(ctx context.Context, opts ...RequestOption) (res *ServerResponse, err error) {
	r := &request{
		method:   http.MethodGet,
		endpoint: "/api/v3/historical-pnl",
		secType:  secTypeSigned,
	}
	data, err := SendRequest(ctx, opts, r, s, err)
	return GetServerResponse(err, data)
}

func (s *ApexClientRequest) GetYesterdayPnl(ctx context.Context, opts ...RequestOption) (res *ServerResponse, err error) {
	r := &request{
		method:   http.MethodGet,
		endpoint: "/api/v3/yesterday-pnl",
		secType:  secTypeSigned,
	}
	data, err := SendRequest(ctx, opts, r, s, err)
	return GetServerResponse(err, data)
}

func (s *ApexClientRequest) GetHistoryValue(ctx context.Context, opts ...RequestOption) (res *ServerResponse, err error) {
	r := &request{
		method:   http.MethodGet,
		endpoint: "/api/v3/history-value",
		secType:  secTypeSigned,
	}
	data, err := SendRequest(ctx, opts, r, s, err)
	return GetServerResponse(err, data)
}

type AccountSnapshot struct {
	Data     AccountSnapshotData `json:"data"`
	TimeCost int64               `json:"timeCost"`
}

type ContractWalletData struct {
	AccountID                string `json:"accountId"`
	Balance                  string `json:"balance"`
	PendingDepositAmount     string `json:"pendingDepositAmount"`
	PendingTransferInAmount  string `json:"pendingTransferInAmount"`
	PendingTransferOutAmount string `json:"pendingTransferOutAmount"`
	PendingWithdrawAmount    string `json:"pendingWithdrawAmount"`
	Token                    string `json:"token"`
}

type OrderData struct {
	AccountID              string      `json:"accountId"`
	CancelReason           string      `json:"cancelReason"`
	ClientID               string      `json:"clientId"`
	CreatedAt              int64       `json:"createdAt"`
	CumSuccessFillFee      string      `json:"cumSuccessFillFee"`
	CumSuccessFillSize     string      `json:"cumSuccessFillSize"`
	CumSuccessFillValue    string      `json:"cumSuccessFillValue"`
	CumSuccessLiquidateFee string      `json:"cumSuccessLiquidateFee"`
	ExpiresAt              int64       `json:"expiresAt"`
	ID                     string      `json:"id"`
	IsDeleverage           bool        `json:"isDeleverage"`
	IsLiquidate            bool        `json:"isLiquidate"`
	IsPositionTpsl         bool        `json:"isPositionTpsl"`
	IsSetOpenSl            bool        `json:"isSetOpenSl"`
	IsSetOpenTp            bool        `json:"isSetOpenTp"`
	LimitFee               string      `json:"limitFee"`
	OpenSlParams           TriggerData `json:"openSlParams"`
	OpenTpParams           TriggerData `json:"openTpParams"`
	Price                  string      `json:"price"`
	ReduceOnly             bool        `json:"reduceOnly"`
	RemainingSize          string      `json:"remainingSize"`
	Side                   string      `json:"side"`
	Size                   string      `json:"size"`
	Status                 string      `json:"status"`
	Symbol                 string      `json:"symbol"`
	TimeInForce            string      `json:"timeInForce"`
	TriggerPrice           string      `json:"triggerPrice"`
	TriggerPriceType       string      `json:"triggerPriceType"`
	Type                   string      `json:"type"`
	UpdatedAt              int64       `json:"updatedAt"`
}

type PerpAccountData struct {
	AccountID             string `json:"accountId"`
	CreatedAt             int64  `json:"createdAt"`
	L2Key                 string `json:"l2Key"`
	MakerFeeRate          string `json:"makerFeeRate"`
	Status                string `json:"status"`
	TakerFeeRate          string `json:"takerFeeRate"`
	UnrealizePnlPriceType string `json:"unrealizePnlPriceType"`
	UpdatedAt             int64  `json:"updatedAt"`
	UserID                string `json:"userId"`
	VipMakerFeeRate       string `json:"vipMakerFeeRate"`
	VipTakerFeeRate       string `json:"vipTakerFeeRate"`
}

type PositionClosedTransactionData struct {
	AccountID             string `json:"accountId"`
	CloseSharedFundingFee string `json:"closeSharedFundingFee"`
	CloseSharedOpenFee    string `json:"closeSharedOpenFee"`
	CloseSharedOpenValue  string `json:"closeSharedOpenValue"`
	CreatedAt             int64  `json:"createdAt"`
	Fee                   string `json:"fee"`
	FundingValue          string `json:"fundingValue"`
	ID                    string `json:"id"`
	IsDeleverage          bool   `json:"isDeleverage"`
	IsLiquidate           bool   `json:"isLiquidate"`
	LiquidateFee          string `json:"liquidateFee"`
	OrderID               string `json:"orderId"`
	Price                 string `json:"price"`
	Side                  string `json:"side"`
	Size                  string `json:"size"`
	Symbol                string `json:"symbol"`
	Type                  string `json:"type"`
}

type PositionData struct {
	AccountID          string `json:"accountId"`
	CustomImr          string `json:"customImr"`
	EntryPrice         string `json:"entryPrice"`
	ExitPrice          string `json:"exitPrice"`
	FundingFee         string `json:"fundingFee"`
	OpenValue          string `json:"openValue"`
	RealizedPnl        string `json:"realizedPnl"`
	Side               string `json:"side"`
	Size               string `json:"size"`
	SumClose           string `json:"sumClose"`
	SumOpen            string `json:"sumOpen"`
	Symbol             string `json:"symbol"`
	TotalCumCloseFee   string `json:"totalCumCloseFee"`
	TotalCumCloseSize  string `json:"totalCumCloseSize"`
	TotalCumCloseValue string `json:"totalCumCloseValue"`
	TotalCumFundingFee string `json:"totalCumFundingFee"`
	TotalCumOpenFee    string `json:"totalCumOpenFee"`
	TotalCumOpenSize   string `json:"totalCumOpenSize"`
	TotalCumOpenValue  string `json:"totalCumOpenValue"`
	UpdatedAt          int64  `json:"updatedAt"`
}

type AccountSnapshotData struct {
	ContractAccounts           []PerpAccountData               `json:"contractAccounts"`
	ContractWallets            []ContractWalletData            `json:"contractWallets"`
	Orders                     []OrderData                     `json:"orders"`
	PositionClosedTransactions []PositionClosedTransactionData `json:"positionClosedTransactions"`
	Positions                  []PositionData                  `json:"positions"`
}

type TriggerData struct {
	TriggerPrice     string `json:"triggerPrice"`
	TriggerPriceType string `json:"triggerPriceType"`
	TriggerSize      string `json:"triggerSize"`
}

type AccountInfo struct {
	AvailableBalance          string `json:"availableBalance"`
	InitialMargin             string `json:"initialMargin"`
	MaintenanceMargin         string `json:"maintenanceMargin"`
	RealizedPnl               string `json:"realizedPnl"`
	TotalEquityValue          string `json:"totalEquityValue"`
	TotalRisk                 string `json:"totalRisk"`
	TotalValueWithoutDiscount string `json:"totalValueWithoutDiscount"`
	UnrealizedPnl             string `json:"unrealizedPnl"`
	WalletBalance             string `json:"walletBalance"`
}

func (s *ApexClientRequest) GetAccountSnapShot(ctx context.Context, opts ...RequestOption) (res *ServerResponse, err error) {
	r := &request{
		method:   http.MethodGet,
		endpoint: "/api/v3/account-snapshot",
		secType:  secTypeSigned,
	}
	data, err := SendRequest(ctx, opts, r, s, err)
	return GetServerResponse(err, data)
}

func (s *ApexClientRequest) SetInitialMarginRate(ctx context.Context, opts ...RequestOption) (res *ServerResponse, err error) {
	r := &request{
		method:   http.MethodPost,
		endpoint: "/api/v3/set-initial-margin-rate",
		secType:  secTypeSigned,
	}
	data, err := SendRequest(ctx, opts, r, s, err)
	return GetServerResponse(err, data)
}
