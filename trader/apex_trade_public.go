package trader

import (
	"context"
	"net/http"
)

func (s *ApexClientRequest) GetSymbols(ctx context.Context, opts ...RequestOption) (res *ServerResponse, err error) {
	r := &request{
		method:   http.MethodGet,
		endpoint: "/api/v3/symbols",
		secType:  secTypeNone,
	}
	data, err := SendRequest(ctx, opts, r, s, err)
	return GetServerResponse(err, data)
}

type AllTickDatas []TickDtaa

type TickDtaa struct {
	FundingRate          string `json:"fundingRate"`
	HighPrice24h         string `json:"highPrice24h"`
	IconURL              string `json:"iconUrl"`
	IndexPrice           string `json:"indexPrice"`
	LastPrice            string `json:"lastPrice"`
	LowPrice24h          string `json:"lowPrice24h"`
	MarkPrice            string `json:"markPrice"`
	NextFundingTime      string `json:"nextFundingTime"`
	OpenInterest         string `json:"openInterest"`
	OraclePrice          string `json:"oraclePrice"`
	PredictedFundingRate string `json:"predictedFundingRate"`
	Price24hPcnt         string `json:"price24hPcnt"`
	Symbol               string `json:"symbol"`
	TradeCount           string `json:"tradeCount"`
	Turnover24h          string `json:"turnover24h"`
	Volume24h            string `json:"volume24h"`
}

func (s *ApexClientRequest) GetAllTicksData(ctx context.Context, opts ...RequestOption) (res *ServerResponse, err error) {
	r := &request{
		method:   http.MethodGet,
		endpoint: "/api/v3/data/all-ticker-mixture",
		secType:  secTypeNone,
	}
	data, err := SendRequest(ctx, opts, r, s, err)
	return GetServerResponse(err, data)
}
