package trader

const (
	Name    = "nofx.api.go"
	Version = "1.0.0"
	// Https
	MAINNET  = "https://omni.apex.exchange"
	TESTNET  = "https://qa.omni.apex.exchange"
	LOCALNET = "http://localhost:8080"

	MAIN_WEBSOCKET_PUBLIC = "wss://quote.omni.apex.exchange/realtime_public"
	TEST_WEBSOCKET_PUBLIC = "wss://qa-quote.omni.apex.exchange/realtime_public"

	MAIN_WEBSOCKET_PRIVATE = "wss://quote.omni.apex.exchange/realtime_private"
	TEST_WEBSOCKET_PRIVATE = "wss://qa-quote.omni.apex.exchange/realtime_private"

	// Globals
	timestampKey  = "APEX-TIMESTAMP"
	signatureKey  = "APEX-SIGNATURE"
	apiRequestKey = "APEX-API-KEY"
	passphraseKey = "APEX-PASSPHRASE"

	NETWORKID_MAIN = 1
	NETWORKID_TEST = 11155111

	NETWORKID_OMNI_MAIN_ARB  = 9
	NETWORKID_OMNI_TEST_BNB  = 3
	NETWORKID_OMNI_TEST_BASE = 11

	ECOrder10 = "3618502788666131213697322783095070105526743751716087489154079457884512865583"
)

type ApexMetaConfig struct {
	ContractConfig ContractConfigData `json:"contractConfig"`
}

type ContractConfigData struct {
	PerpetualContract []PerpetualContract `json:"perpetualContract"`
	StockContract     []PerpetualContract `json:"stockContract"`
}

type PerpetualContract struct {
	BaselinePositionValue            string          `json:"baselinePositionValue"`
	CrossID                          int             `json:"crossId"`
	CrossSymbolID                    int             `json:"crossSymbolId"`
	CrossSymbolName                  string          `json:"crossSymbolName"`
	DigitMerge                       string          `json:"digitMerge"`
	DisplayMaxLeverage               string          `json:"displayMaxLeverage"`
	DisplayMinLeverage               string          `json:"displayMinLeverage"`
	EnableDisplay                    bool            `json:"enableDisplay"`
	EnableOpenPosition               bool            `json:"enableOpenPosition"`
	EnableTrade                      bool            `json:"enableTrade"`
	FundingImpactMarginNotional      string          `json:"fundingImpactMarginNotional"`
	FundingInterestRate              string          `json:"fundingInterestRate"`
	IncrementalInitialMarginRate     string          `json:"incrementalInitialMarginRate"`
	IncrementalMaintenanceMarginRate string          `json:"incrementalMaintenanceMarginRate"`
	IncrementalPositionValue         string          `json:"incrementalPositionValue"`
	InitialMarginRate                string          `json:"initialMarginRate"`
	MaintenanceMarginRate            string          `json:"maintenanceMarginRate"`
	MaxOrderSize                     string          `json:"maxOrderSize"`
	MaxPositionSize                  string          `json:"maxPositionSize"`
	MinOrderSize                     string          `json:"minOrderSize"`
	MaxMarketPriceRange              string          `json:"maxMarketPriceRange"`
	SettleAssetID                    string          `json:"settleAssetId"`
	BaseTokenID                      string          `json:"baseTokenId"`
	StepSize                         string          `json:"stepSize"`
	Symbol                           string          `json:"symbol"`
	SymbolDisplayName                string          `json:"symbolDisplayName"`
	TickSize                         string          `json:"tickSize"`
	MaxMaintenanceMarginRate         string          `json:"maxMaintenanceMarginRate"`
	MaxPositionValue                 string          `json:"maxPositionValue"`
	TagIconURL                       string          `json:"tagIconUrl"`
	Tag                              string          `json:"tag"`
	RiskTip                          bool            `json:"riskTip"`
	DefaultInitialMarginRate         string          `json:"defaultInitialMarginRate"`
	KlineStartTime                   int             `json:"klineStartTime"`
	MaxMarketSizeBuffer              string          `json:"maxMarketSizeBuffer"`
	EnableFundingSettlement          bool            `json:"enableFundingSettlement"`
	IndexPriceDecimals               int             `json:"indexPriceDecimals"`
	IndexPriceVarRate                string          `json:"indexPriceVarRate"`
	OpenPositionOiLimitRate          string          `json:"openPositionOiLimitRate"`
	FundingMaxRate                   string          `json:"fundingMaxRate"`
	FundingMinRate                   string          `json:"fundingMinRate"`
	FundingMaxValue                  string          `json:"fundingMaxValue"`
	EnableFundingMxValue             bool            `json:"enableFundingMxValue"`
	L2PairID                         string          `json:"l2PairId"`
	SettleTimeStamp                  int             `json:"settleTimeStamp"`
	IsPrelaunch                      bool            `json:"isPrelaunch"`
	RiskLimitConfig                  RiskLimitConfig `json:"riskLimitConfig"`
}

type RiskLimitConfig struct {
	PositionSteps []string `json:"positionSteps"`
	IMRSteps      []string `json:"imrSteps"`
	MMRSteps      []string `json:"mmrSteps"`
}
