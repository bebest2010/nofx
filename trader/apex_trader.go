package trader

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math"
	"math/big"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/shopspring/decimal"

	"nofx/libs/zklink"
)

const (
	stopLimitStr        = "STOP_LIMIT"
	stopMarketStr       = "STOP_MARKET"
	takeProfitLimitStr  = "TAKE_PROFIT_LIMIT"
	takeProfitMarketStr = "TAKE_PROFIT_MARKET"
)

// ApexTrader Apex交易平台实现
type ApexTrader struct {
	ctx             context.Context
	zklinkOmniSeeds string
	apikey          string
	secret          string
	passphrase      string
	client          *http.Client
	baseURL         string

	// 缓存交易对精度信息
	symbolPrecision map[string]ApexSymbolConfig
	mu              sync.RWMutex // for symbol

	// for account
	muAccount   sync.RWMutex // for symbol
	accountID   string
	takeFeeRate string

	muPrice        sync.RWMutex // for symbol price
	priceInfoCache map[string]PriceInfo
	cacheTime      int64
}

type PriceInfo struct {
	LastPrice  float64
	MarkPrice  float64
	IndexPrice float64
}

// SymbolPrecision 交易对精度信息
type ApexSymbolConfig struct {
	PricePrecision    int
	QuantityPrecision int
	TickSize          float64 // 价格步进值
	StepSize          float64 // 数量步进值
	PairId            uint32
	Mmr               string
	MaxOrderSize      string
	MaxPositionSize   string
	IsStockToken      bool
}

// NewApexTrader 创建Apex交易器
// omniSeeds zklink 签名的seeds 从 apex前端api管理 Export omniseeds 获取
// apikey apikey 从 apex前端api管理 Export apikey 获取
// secret secret 从 apex前端api管理 Export secret 获取
// passphrase passphrase 从 apex前端api管理 Export passphrase 获取

func NewApexTrader(zklinkOmniSeeds, apikey, secret, passphrase string, bTestnet bool) (*ApexTrader, error) {

	client := &http.Client{
		Timeout: 30 * time.Second, // 增加到30秒
		Transport: &http.Transport{
			TLSHandshakeTimeout:   10 * time.Second,
			ResponseHeaderTimeout: 10 * time.Second,
			IdleConnTimeout:       90 * time.Second,
		},
	}
	baseUrl := "https://qa.omni.apex.exchange"
	if !bTestnet {
		baseUrl = "https://omni.apex.exchange"
	}
	return &ApexTrader{
		ctx:             context.Background(),
		zklinkOmniSeeds: zklinkOmniSeeds,
		apikey:          apikey,
		secret:          secret,
		passphrase:      passphrase,
		symbolPrecision: make(map[string]ApexSymbolConfig),
		priceInfoCache:  make(map[string]PriceInfo),
		client:          client,
		baseURL:         baseUrl,
	}, nil
}

// genNonce 生成微秒时间戳
func (t *ApexTrader) genNonce() uint64 {
	return uint64(time.Now().UnixMicro())
}

// getPrecision 获取交易对精度信息
func (t *ApexTrader) getPrecision(symbol string) (ApexSymbolConfig, error) {
	t.mu.RLock()
	if prec, ok := t.symbolPrecision[symbol]; ok {
		t.mu.RUnlock()
		return prec, nil
	}
	t.mu.RUnlock()

	// 获取交易所信息
	client := NewApexHttpClient(t.baseURL, t.apikey, t.secret, t.passphrase, WithBaseURL(t.baseURL), WithDebug(true))
	retData, err := client.NewUtaApexServiceNoParams().GetSymbols(context.TODO())
	if err != nil {
		return ApexSymbolConfig{}, err
	}
	metaData, err := json.Marshal(retData.Data)
	if err != nil {
		return ApexSymbolConfig{}, err
	}
	var info ApexMetaConfig
	// fmt.Println("Get symbols response result:", string(metaData))
	if err := json.Unmarshal(metaData, &info); err != nil {
		return ApexSymbolConfig{}, err
	}

	// 缓存所有交易对的精度
	t.mu.Lock()
	for _, s := range info.ContractConfig.PerpetualContract {
		decimalsTick, errParse := decimal.NewFromString(s.TickSize)
		if errParse != nil {
			continue
		}
		decimalsStep, errParse := decimal.NewFromString(s.StepSize)
		if errParse != nil {
			continue
		}

		prec := ApexSymbolConfig{
			PricePrecision:    int(0 - decimalsTick.Exponent()),
			QuantityPrecision: int(0 - decimalsStep.Exponent()),

			Mmr:             s.MaintenanceMarginRate,
			MaxOrderSize:    s.MaxOrderSize,
			MaxPositionSize: s.MaxPositionSize,
			IsStockToken:    false,
		}
		pairId, _ := strconv.ParseUint(s.L2PairID, 10, 32)
		prec.PairId = uint32(pairId)
		prec.TickSize, _ = decimalsTick.Float64()
		prec.StepSize, _ = decimalsStep.Float64()

		t.symbolPrecision[s.Symbol] = prec
		fmt.Printf("Cached precision for symbol:%v, data %#v\n", s.Symbol, prec)
	}

	for _, s := range info.ContractConfig.StockContract {
		decimalsTick, errParse := decimal.NewFromString(s.TickSize)
		if errParse != nil {
			continue
		}
		decimalsStep, errParse := decimal.NewFromString(s.StepSize)
		if errParse != nil {
			continue
		}

		prec := ApexSymbolConfig{
			PricePrecision:    int(0 - decimalsTick.Exponent()),
			QuantityPrecision: int(0 - decimalsStep.Exponent()),
			Mmr:               s.MaintenanceMarginRate,
			MaxOrderSize:      s.MaxOrderSize,
			MaxPositionSize:   s.MaxPositionSize,
			IsStockToken:      true,
		}
		pairId, _ := strconv.ParseUint(s.L2PairID, 10, 32)
		prec.PairId = uint32(pairId)
		prec.TickSize, _ = decimalsTick.Float64()
		prec.StepSize, _ = decimalsStep.Float64()

		t.symbolPrecision[s.Symbol] = prec
	}
	t.mu.Unlock()

	if prec, ok := t.symbolPrecision[symbol]; ok {
		return prec, nil
	}

	return ApexSymbolConfig{}, fmt.Errorf("未找到交易对 %s 的精度信息", symbol)
}

// roundToTickSize 将价格/数量四舍五入到tick size/step size的整数倍

// formatPrice 格式化价格到正确精度和tick size
func (t *ApexTrader) formatPrice(symbol string, price float64) (float64, error) {
	prec, err := t.getPrecision(symbol)
	if err != nil {
		return 0, err
	}

	// 优先使用tick size，确保价格是tick size的整数倍
	if prec.TickSize > 0 {
		return roundToTickSize(price, prec.TickSize), nil
	}

	// 如果没有tick size，则按精度四舍五入
	multiplier := math.Pow10(prec.PricePrecision)
	return math.Round(price*multiplier) / multiplier, nil
}

// formatQuantity 格式化数量到正确精度和step size
func (t *ApexTrader) formatQuantity(symbol string, quantity float64) (float64, error) {
	prec, err := t.getPrecision(symbol)
	if err != nil {
		return 0, err
	}

	// 优先使用step size，确保数量是step size的整数倍
	if prec.StepSize > 0 {
		return roundToTickSize(quantity, prec.StepSize), nil
	}

	// 如果没有step size，则按精度四舍五入
	multiplier := math.Pow10(prec.QuantityPrecision)
	return math.Round(quantity*multiplier) / multiplier, nil
}

// formatFloatWithPrecision 将浮点数格式化为指定精度的字符串（去除末尾的0）
func (t *ApexTrader) formatFloatWithPrecision(value float64, precision int) string {
	// 使用指定精度格式化
	formatted := strconv.FormatFloat(value, 'f', precision, 64)

	// 去除末尾的0和小数点（如果有）
	formatted = strings.TrimRight(formatted, "0")
	formatted = strings.TrimRight(formatted, ".")

	return formatted
}

func (t *ApexTrader) GetAccountData() error {
	t.muAccount.RLock()
	account := t.accountID
	feeRate := t.takeFeeRate
	t.muAccount.RUnlock()
	if account != "" && feeRate != "" {
		return nil
	}
	client := NewApexHttpClient(t.baseURL, t.apikey, t.secret, t.passphrase, WithBaseURL(t.baseURL), WithDebug(true))
	accountResult, err := client.NewUtaApexServiceNoParams().GetAccountSnapShot(context.Background())
	if err != nil {
		fmt.Println(err)
		return err
	}
	// fmt.Println(PrettyPrint(accountResult))
	if accountResult.Code != 0 {
		return errors.New(accountResult.Message)
	}
	accountData, _ := json.Marshal(accountResult.Data)
	if accountData == nil {
		return errors.New("empty account data")
	}
	var snapshot AccountSnapshotData
	err = json.Unmarshal([]byte(accountData), &snapshot)
	if err != nil {
		return err
	}
	// fmt.Println(PrettyPrint(snapshot))

	t.muAccount.Lock()
	t.accountID = snapshot.ContractAccounts[0].AccountID
	t.takeFeeRate = snapshot.ContractAccounts[0].TakerFeeRate
	t.muAccount.Unlock()

	fmt.Printf("t.accountID %v, t.takeFeeRate %v\n", t.accountID, t.takeFeeRate)
	return nil
}

// GetBalance 获取账户余额
func (t *ApexTrader) GetBalance() (map[string]interface{}, error) {
	client := NewApexHttpClient(t.baseURL, t.apikey, t.secret, t.passphrase, WithBaseURL(t.baseURL), WithDebug(true))
	accountResult, err := client.NewUtaApexServiceNoParams().GetAccountBalance(context.Background())
	if err != nil {
		fmt.Println(err)
		return nil, err
	}
	// fmt.Println(PrettyPrint(accountResult))
	// 查找USDT余额
	if accountResult.Code != 0 {
		return nil, errors.New(accountResult.Message)
	}
	accountData, _ := json.Marshal(accountResult.Data)
	if accountData == nil {
		return nil, errors.New("empty account data")
	}

	var accontInfo AccountInfo
	err = json.Unmarshal([]byte(accountData), &accontInfo)
	if err != nil {
		return nil, err
	}
	totalBalance, _ := strconv.ParseFloat(accontInfo.TotalValueWithoutDiscount, 64)
	availableBalance, _ := strconv.ParseFloat(accontInfo.AvailableBalance, 64)
	totalUnrealizedProfit, _ := strconv.ParseFloat(accontInfo.UnrealizedPnl, 64)
	return map[string]interface{}{
		"totalWalletBalance":    totalBalance,          // 钱包余额
		"availableBalance":      availableBalance,      // 可用余额
		"totalUnrealizedProfit": totalUnrealizedProfit, // 未实现盈亏
	}, nil
}

// GetPositions 获取持仓信息
func (t *ApexTrader) GetPositions() ([]map[string]interface{}, error) {
	client := NewApexHttpClient(t.baseURL, t.apikey, t.secret, t.passphrase, WithBaseURL(t.baseURL), WithDebug(true))
	accountResult, err := client.NewUtaApexServiceNoParams().GetAccountSnapShot(context.Background())
	if err != nil {
		fmt.Println(err)
		return nil, err
	}
	// fmt.Println(PrettyPrint(accountResult))
	if accountResult.Code != 0 {
		return nil, errors.New(accountResult.Message)
	}
	accountData, _ := json.Marshal(accountResult.Data)
	if accountData == nil {
		return nil, errors.New("empty account data")
	}

	var snapshot AccountSnapshotData
	err = json.Unmarshal([]byte(accountData), &snapshot)
	if err != nil {
		return nil, err
	}
	// 获取下账号信息
	accountBalance, err := t.GetBalance()
	if err != nil {
		return nil, err
	}
	remainBalance := accountBalance["totalWalletBalance"].(float64)
	result := []map[string]interface{}{}
	for _, pos := range snapshot.Positions {

		posAmt, _ := strconv.ParseFloat(pos.Size, 64)
		if posAmt == 0 {
			continue // 跳过空仓位
		}
		markPriceInfo, err := t.getMarketPriceInfo(pos.Symbol)
		if err != nil {
			log.Printf("getMarketPriceInfo for sym %v, %v", pos.Symbol, err)
		}
		entryPrice, _ := strconv.ParseFloat(pos.EntryPrice, 64)
		markPrice := markPriceInfo.MarkPrice

		//  unRealizedProfit 计算
		unRealizedProfit := 0.0
		if pos.Side == "LONG" || pos.Side == "long" {
			unRealizedProfit = (markPrice - entryPrice) * posAmt
		} else {
			unRealizedProfit = (entryPrice - markPrice) * posAmt
		}
		leverageVal := 1.0
		imrVal, _ := strconv.ParseFloat(pos.CustomImr, 64)
		if imrVal > 0.0 {
			leverageVal = 1.0 / imrVal
		}
		realizedProfit, _ := strconv.ParseFloat(pos.RealizedPnl, 64)

		// 粗略计算下 强评价格，使用 剩余保证金，计算全部亏损完的价格
		priceDiff := remainBalance / posAmt
		liquidationPrice := 0.0
		if pos.Side == "LONG" || pos.Side == "long" {
			liquidationPrice = markPrice - priceDiff
		} else {
			liquidationPrice = markPrice + priceDiff
		}
		if liquidationPrice < 0.0 {
			liquidationPrice = 0.0
		}
		// 返回与Binance相同的字段名
		result = append(result, map[string]interface{}{
			"symbol":           pos.Symbol,
			"side":             strings.ToLower(pos.Side), // 保持小写
			"positionAmt":      posAmt,
			"entryPrice":       entryPrice,
			"markPrice":        markPrice,
			"unRealizedProfit": unRealizedProfit,
			"realizedProfit":   realizedProfit,
			"leverage":         leverageVal,
			"liquidationPrice": liquidationPrice,
		})
	}

	return result, nil
}

func (t *ApexTrader) GenerateClientId(timeUnix int64) string {
	randNum, _ := rand.Int(rand.Reader, big.NewInt(1000000))
	return fmt.Sprintf("apexomni-nofx-%v-%v", timeUnix, randNum.Int64())
}

func (t *ApexTrader) PlaceLimitOrder(symbol string, pairId uint32, priceStr, qtyStr string, isLong bool, isIoc, isReduceOnly bool) (uint64, error) {
	if err := t.GetAccountData(); err != nil {
		log.Printf("  ⚠ 获取账号信息失败: %v", err)
		return 0, err
	}

	// clientId=apexomni-588981506747138153-1763535130571-298207&expiration=1766127129494&limitFee=0.000500&price=1.0000&reduceOnly=false&side=BUY&signature=ae4ae1f0ec1c7d03510550afcfc9ce12a6aa5a6eb7f762249c8bc743617d280e47a1d940b4c6f6c942e86e07ecb8290a21f0a7caa38e5e66aba469d3c5ed5a02&size=1&symbol=XRP-USDT&timeInForce=GOOD_TIL_CANCEL&type=LIMIT
	unixTimeMs := time.Now().UnixMilli()
	clientId := t.GenerateClientId(unixTimeMs)
	expireTime := unixTimeMs + 30*24*3600*1000

	// limitFee = price * size * takerFeeRate
	limitFee := fmt.Sprintf("%.6f", decimal.RequireFromString(t.takeFeeRate).Mul(decimal.RequireFromString(priceStr)).Mul(decimal.RequireFromString(qtyStr)).InexactFloat64())

	// signatuer calc
	sign, err := zklink.NewZkSignerFromSeeds([]byte(t.zklinkOmniSeeds))
	if err != nil {
		return 0, err
	}
	// SignOrder(accountIdOrig, clientId, symbol, size, price string, pairID uint32, isBuy bool, takeFeeRate, makeFeeRate string) (string, error)

	signature, err := sign.SignOrder(t.accountID, clientId, symbol, qtyStr, priceStr, uint32(pairId), true, t.takeFeeRate, t.takeFeeRate)
	if err != nil {
		return 0, err
	}

	side := "BUY"
	if !isLong {
		side = "SELL"
	}
	timeInForce := "GOOD_TIL_CANCEL"
	if isIoc {
		timeInForce = "IMMEDIATE_OR_CANCEL"
	}
	params := map[string]interface{}{
		"clientId":    clientId,
		"expiration":  fmt.Sprintf("%v", expireTime),
		"limitFee":    limitFee,
		"price":       priceStr,
		"reduceOnly":  isReduceOnly,
		"side":        side,
		"size":        qtyStr,
		"symbol":      symbol,
		"type":        "LIMIT",
		"signature":   signature,
		"timeInForce": timeInForce,
	}

	client := NewApexHttpClient(t.baseURL, t.apikey, t.secret, t.passphrase, WithBaseURL(t.baseURL), WithDebug(true))

	orderResult, err := client.NewUtaApexServiceWithParams(params).CreateOrder(context.Background())
	if err != nil {
		return 0, err
	}

	if orderResult.Code != 0 {
		return 0, errors.New(orderResult.Message)
	}
	marshalData, _ := json.Marshal(orderResult.Data)
	if marshalData == nil {
		return 0, errors.New("empty data")
	}

	var orderRetData OrderData
	err = json.Unmarshal([]byte(marshalData), &orderRetData)
	if err != nil {
		return 0, err
	}
	return strconv.ParseUint(orderRetData.ID, 10, 64)
}

// OpenLong 开多单
func (t *ApexTrader) OpenLong(symbol string, quantity float64, leverage int) (map[string]interface{}, error) {
	// 开仓前先取消所有挂单,防止残留挂单导致仓位叠加
	if err := t.CancelAllOrders(symbol); err != nil {
		log.Printf("  ⚠ 取消挂单失败(继续开仓): %v", err)
	}

	// 先设置杠杆
	if err := t.SetLeverage(symbol, leverage); err != nil {
		return nil, fmt.Errorf("设置杠杆失败: %w", err)
	}

	// 获取当前价格
	price, err := t.GetMarketPrice(symbol)
	if err != nil {
		log.Printf("  ⚠ GetMarketPrice failed: %v", err)
		return nil, err
	}

	// 使用限价单模拟市价单（价格设置得稍高一些以确保成交）
	limitPrice := price * 1.05

	// 格式化价格和数量到正确精度
	formattedPrice, err := t.formatPrice(symbol, limitPrice)
	if err != nil {
		return nil, err
	}
	formattedQty, err := t.formatQuantity(symbol, quantity)
	if err != nil {
		return nil, err
	}

	// 获取精度信息
	prec, err := t.getPrecision(symbol)
	if err != nil {
		log.Printf("  ⚠ getPrecision failed: %v", err)
		return nil, err
	}

	// 转换为字符串，使用正确的精度格式
	priceStr := t.formatFloatWithPrecision(formattedPrice, prec.PricePrecision)
	qtyStr := t.formatFloatWithPrecision(formattedQty, prec.QuantityPrecision)

	log.Printf("  📏 精度处理: 价格 %.8f -> %s (精度=%d), 数量 %.8f -> %s (精度=%d)",
		limitPrice, priceStr, prec.PricePrecision, quantity, qtyStr, prec.QuantityPrecision)

	orderID, err := t.PlaceLimitOrder(symbol, prec.PairId, priceStr, qtyStr, true, false, false)
	if err != nil {
		log.Printf("  ⚠ PlaceLimitOrder失败: %v", err)
		return nil, err
	}
	result := make(map[string]interface{})
	result["symbol"] = symbol
	result["orderId"] = orderID
	result["status"] = "FILLED" // 价格高 默认是filled
	return result, nil
}

// OpenShort 开空单
func (t *ApexTrader) OpenShort(symbol string, quantity float64, leverage int) (map[string]interface{}, error) {

	// 开仓前先取消所有挂单,防止残留挂单导致仓位叠加
	if err := t.CancelAllOrders(symbol); err != nil {
		log.Printf("  ⚠ 取消挂单失败(继续开仓): %v", err)
	}

	// 先设置杠杆
	if err := t.SetLeverage(symbol, leverage); err != nil {
		return nil, fmt.Errorf("设置杠杆失败: %w", err)
	}

	// 获取当前价格
	price, err := t.GetMarketPrice(symbol)
	if err != nil {
		return nil, err
	}

	// 使用限价单模拟市价单（价格设置得稍低一些以确保成交）
	limitPrice := price * 0.95

	// 格式化价格和数量到正确精度
	formattedPrice, err := t.formatPrice(symbol, limitPrice)
	if err != nil {
		return nil, err
	}
	formattedQty, err := t.formatQuantity(symbol, quantity)
	if err != nil {
		return nil, err
	}

	// 获取精度信息
	prec, err := t.getPrecision(symbol)
	if err != nil {
		return nil, err
	}

	// 转换为字符串，使用正确的精度格式
	priceStr := t.formatFloatWithPrecision(formattedPrice, prec.PricePrecision)
	qtyStr := t.formatFloatWithPrecision(formattedQty, prec.QuantityPrecision)

	log.Printf("  📏 精度处理: 价格 %.8f -> %s (精度=%d), 数量 %.8f -> %s (精度=%d)",
		limitPrice, priceStr, prec.PricePrecision, quantity, qtyStr, prec.QuantityPrecision)

	isLong := false
	orderID, err := t.PlaceLimitOrder(symbol, prec.PairId, priceStr, qtyStr, isLong, false, false)
	if err != nil {
		log.Printf("  ⚠ PlaceLimitOrder失败: %v", err)
		return nil, err
	}
	result := make(map[string]interface{})
	result["symbol"] = symbol
	result["orderId"] = orderID
	result["status"] = "FILLED" // 价格高 默认是filled
	return result, nil
}

// CloseLong 平多单
func (t *ApexTrader) CloseLong(symbol string, quantity float64) (map[string]interface{}, error) {

	// 如果数量为0，获取当前持仓数量
	if quantity == 0 {
		positions, err := t.GetPositions()
		if err != nil {
			return nil, err
		}

		for _, pos := range positions {
			if pos["symbol"] == symbol && pos["side"] == "long" {
				quantity = pos["positionAmt"].(float64)
				break
			}
		}

		if quantity == 0 {
			return nil, fmt.Errorf("没有找到 %s 的多仓", symbol)
		}
		log.Printf("  📊 获取到多仓数量: %.8f", quantity)
	}

	price, err := t.GetMarketPrice(symbol)
	if err != nil {
		return nil, err
	}

	limitPrice := price * 0.95

	// 格式化价格和数量到正确精度
	formattedPrice, err := t.formatPrice(symbol, limitPrice)
	if err != nil {
		return nil, err
	}
	formattedQty, err := t.formatQuantity(symbol, quantity)
	if err != nil {
		return nil, err
	}

	// 获取精度信息
	prec, err := t.getPrecision(symbol)
	if err != nil {
		return nil, err
	}

	// 转换为字符串，使用正确的精度格式
	priceStr := t.formatFloatWithPrecision(formattedPrice, prec.PricePrecision)
	qtyStr := t.formatFloatWithPrecision(formattedQty, prec.QuantityPrecision)

	log.Printf("  📏 精度处理: 价格 %.8f -> %s (精度=%d), 数量 %.8f -> %s (精度=%d)",
		limitPrice, priceStr, prec.PricePrecision, quantity, qtyStr, prec.QuantityPrecision)

	isLong := false
	orderID, err := t.PlaceLimitOrder(symbol, prec.PairId, priceStr, qtyStr, isLong, false, false)
	if err != nil {
		log.Printf("  ⚠ PlaceLimitOrder失败: %v", err)
		return nil, err
	}

	log.Printf("✓ 平多仓成功: %s 数量: %s", symbol, qtyStr)

	// 平仓后取消该币种的所有挂单(止损止盈单)
	if err := t.CancelAllOrders(symbol); err != nil {
		log.Printf("  ⚠ 取消挂单失败: %v", err)
	}

	result := make(map[string]interface{})
	result["symbol"] = symbol
	result["orderId"] = orderID
	result["status"] = "FILLED" // 价格高 默认是filled
	return result, nil
}

// CloseShort 平空单
func (t *ApexTrader) CloseShort(symbol string, quantity float64) (map[string]interface{}, error) {

	// 如果数量为0，获取当前持仓数量
	if quantity == 0 {
		positions, err := t.GetPositions()
		if err != nil {
			return nil, err
		}

		for _, pos := range positions {
			if pos["symbol"] == symbol && pos["side"] == "short" {
				// Aster的GetPositions已经将空仓数量转换为正数，直接使用
				quantity = pos["positionAmt"].(float64)
				break
			}
		}

		if quantity == 0 {
			return nil, fmt.Errorf("没有找到 %s 的空仓", symbol)
		}
		log.Printf("  📊 获取到空仓数量: %.8f", quantity)
	}

	price, err := t.GetMarketPrice(symbol)
	if err != nil {
		return nil, err
	}

	limitPrice := price * 1.05

	// 格式化价格和数量到正确精度
	formattedPrice, err := t.formatPrice(symbol, limitPrice)
	if err != nil {
		return nil, err
	}
	formattedQty, err := t.formatQuantity(symbol, quantity)
	if err != nil {
		return nil, err
	}

	// 获取精度信息
	prec, err := t.getPrecision(symbol)
	if err != nil {
		return nil, err
	}

	// 转换为字符串，使用正确的精度格式
	priceStr := t.formatFloatWithPrecision(formattedPrice, prec.PricePrecision)
	qtyStr := t.formatFloatWithPrecision(formattedQty, prec.QuantityPrecision)

	log.Printf("  📏 精度处理: 价格 %.8f -> %s (精度=%d), 数量 %.8f -> %s (精度=%d)",
		limitPrice, priceStr, prec.PricePrecision, quantity, qtyStr, prec.QuantityPrecision)

	isLong := true
	orderID, err := t.PlaceLimitOrder(symbol, prec.PairId, priceStr, qtyStr, isLong, false, false)
	if err != nil {
		log.Printf("  ⚠ PlaceLimitOrder失败: %v", err)
		return nil, err
	}

	log.Printf("✓ 平空仓成功: %s 数量: %s", symbol, qtyStr)

	// 平仓后取消该币种的所有挂单(止损止盈单)
	if err := t.CancelAllOrders(symbol); err != nil {
		log.Printf("  ⚠ 取消挂单失败: %v", err)
	}

	result := make(map[string]interface{})
	result["symbol"] = symbol
	result["orderId"] = orderID
	result["status"] = "FILLED" // 价格高 默认是filled
	return result, nil

}

// SetMarginMode 设置仓位模式
func (t *ApexTrader) SetMarginMode(symbol string, isCrossMargin bool) error {
	// Aster支持仓位模式设置
	// API格式与币安相似：CROSSED(全仓) / ISOLATED(逐仓)
	marginType := "CROSSED"
	if !isCrossMargin {
		return fmt.Errorf("Apex暂不支持逐仓模式")
	}

	log.Printf("  ✓ %s 仓位模式已设置为 %s", symbol, marginType)
	return nil
}

// SetLeverage 设置杠杆倍数
func (t *ApexTrader) SetLeverage(symbol string, leverage int) error {

	client := NewApexHttpClient(t.baseURL, t.apikey, t.secret, t.passphrase, WithBaseURL(t.baseURL), WithDebug(true))

	params := map[string]interface{}{"symbol": symbol, "initialMarginRate": fmt.Sprintf("%.4f", 1.0/float64(leverage))}
	setResult, err := client.NewUtaApexServiceWithParams(params).SetInitialMarginRate(context.Background())
	if err != nil {
		fmt.Println(err)

	}
	fmt.Println(PrettyPrint(setResult))
	if setResult.Code != 0 {
		return errors.New(setResult.Message)
	}

	log.Printf("  ✓ %s 杠杆已设置为 %dx", symbol, leverage)
	return err
}

func SymbolRegular(symbol string) string {
	// remove -
	symbolRemove := strings.ReplaceAll(symbol, "-", "")
	// add usdt -usdt
	return strings.TrimSuffix(symbolRemove, "USDT") + "-USDT"
}
func (t *ApexTrader) getMarketPriceInfo(symbol string) (PriceInfo, error) {
	timeNow := time.Now().Unix()
	// price cache 10s

	var ret PriceInfo
	getData := false
	t.muPrice.RLock()
	if t.cacheTime+10 >= timeNow {
		ret, getData = t.priceInfoCache[symbol]
	} else {
		getData = false
	}
	t.muPrice.RUnlock()
	if getData {
		return ret, nil
	}
	// 获取全部币对信息
	client := NewApexHttpClient(t.baseURL, t.apikey, t.secret, t.passphrase, WithBaseURL(t.baseURL), WithDebug(true))
	marketInfoData, err := client.NewUtaApexServiceNoParams().GetAllTicksData(context.TODO())
	if err != nil {
		return ret, err
	}

	if marketInfoData.Code != 0 {
		return ret, errors.New(marketInfoData.Message)
	}
	marshalData, _ := json.Marshal(marketInfoData.Data)
	if marshalData == nil {
		return ret, errors.New("empty account data")
	}

	var allTicks AllTickDatas
	err = json.Unmarshal([]byte(marshalData), &allTicks)
	if err != nil {
		return ret, err
	}

	bFound := false
	t.muPrice.Lock()
	for _, data := range allTicks {
		dataSym := SymbolRegular(data.Symbol)
		var p PriceInfo
		p.IndexPrice, _ = strconv.ParseFloat(data.IndexPrice, 64)
		p.LastPrice, _ = strconv.ParseFloat(data.LastPrice, 64)
		p.MarkPrice, _ = strconv.ParseFloat(data.MarkPrice, 64)
		t.priceInfoCache[dataSym] = p
		if dataSym == symbol {
			ret = p
			bFound = true
		}
	}
	t.cacheTime = time.Now().Unix()
	t.muPrice.Unlock()
	if !bFound {
		return ret, fmt.Errorf("not found")
	}
	return ret, nil
}

// GetMarketPrice 获取市场价格
func (t *ApexTrader) GetMarketPrice(symbol string) (float64, error) {

	// 使用ticker接口获取当前价格
	priceInfo, err := t.getMarketPriceInfo(symbol)

	return priceInfo.LastPrice, err
}

func (t *ApexTrader) PlaceMarketOrder(symbol string, tickSize float64, pairId uint32, trigglePrice, qtyStr string, isLong bool, isIoc, isReduceOnly, isTakeProfit, isStopLoss bool) (uint64, error) {
	if err := t.GetAccountData(); err != nil {
		log.Printf("  ⚠ 获取账号信息失败: %v", err)
		return 0, err
	}

	// clientId=apexomni-588981506747138153-1763535130571-298207&expiration=1766127129494&limitFee=0.000500&price=1.0000&reduceOnly=false&side=BUY&signature=ae4ae1f0ec1c7d03510550afcfc9ce12a6aa5a6eb7f762249c8bc743617d280e47a1d940b4c6f6c942e86e07ecb8290a21f0a7caa38e5e66aba469d3c5ed5a02&size=1&symbol=XRP-USDT&timeInForce=GOOD_TIL_CANCEL&type=LIMIT
	unixTimeMs := time.Now().UnixMilli()
	clientId := t.GenerateClientId(unixTimeMs)
	expireTime := unixTimeMs + 30*24*3600*1000

	// limitFee = price * size * takerFeeRate
	price := tickSize
	if isLong {
		price = decimal.RequireFromString(trigglePrice).Mul(decimal.NewFromInt(10)).InexactFloat64() // 10 tiggle price
	}

	limitFee := fmt.Sprintf("%.6f", decimal.RequireFromString(t.takeFeeRate).Mul(decimal.NewFromFloat(price)).Mul(decimal.RequireFromString(qtyStr)).InexactFloat64())

	// signatuer calc
	sign, err := zklink.NewZkSignerFromSeeds([]byte(t.zklinkOmniSeeds))
	if err != nil {
		return 0, err
	}
	// SignOrder(accountIdOrig, clientId, symbol, size, price string, pairID uint32, isBuy bool, takeFeeRate, makeFeeRate string) (string, error)
	priceStr := fmt.Sprintf("%v", price)
	signature, err := sign.SignOrder(t.accountID, clientId, symbol, qtyStr, priceStr, uint32(pairId), true, t.takeFeeRate, t.takeFeeRate)
	if err != nil {
		return 0, err
	}

	side := "BUY"
	if !isLong {
		side = "SELL"
	}
	timeInForce := "GOOD_TIL_CANCEL"
	if isIoc {
		timeInForce = "IMMEDIATE_OR_CANCEL"
	}
	typeStr := "MARKET"
	if isTakeProfit {
		typeStr = takeProfitMarketStr
	}
	if isStopLoss {
		typeStr = stopMarketStr
	}
	triggerPriceTypeStr := "MARKET" // 市场价触发
	params := map[string]interface{}{
		"clientId":         clientId,
		"expiration":       fmt.Sprintf("%v", expireTime),
		"limitFee":         limitFee,
		"price":            priceStr,
		"reduceOnly":       isReduceOnly,
		"side":             side,
		"size":             qtyStr,
		"symbol":           symbol,
		"type":             typeStr,
		"signature":        signature,
		"triggerPrice":     trigglePrice,
		"triggerPriceType": triggerPriceTypeStr,
		"timeInForce":      timeInForce,
	}

	client := NewApexHttpClient(t.baseURL, t.apikey, t.secret, t.passphrase, WithBaseURL(t.baseURL), WithDebug(true))

	orderResult, err := client.NewUtaApexServiceWithParams(params).CreateOrder(context.Background())
	if err != nil {
		return 0, err
	}

	if orderResult.Code != 0 {
		return 0, errors.New(orderResult.Message)
	}
	marshalData, _ := json.Marshal(orderResult.Data)
	if marshalData == nil {
		return 0, errors.New("empty data")
	}

	var orderRetData OrderData
	err = json.Unmarshal([]byte(marshalData), &orderRetData)
	if err != nil {
		return 0, err
	}
	return strconv.ParseUint(orderRetData.ID, 10, 64)
}

// SetStopLoss 设置止损
func (t *ApexTrader) SetStopLoss(symbol string, positionSide string, quantity, stopPrice float64) error {

	isLong := false

	if positionSide == "SHORT" {
		isLong = true
	}

	// 格式化价格和数量到正确精度
	formattedPrice, err := t.formatPrice(symbol, stopPrice)
	if err != nil {
		return err
	}
	formattedQty, err := t.formatQuantity(symbol, quantity)
	if err != nil {
		return err
	}

	// 获取精度信息
	prec, err := t.getPrecision(symbol)
	if err != nil {
		return err
	}

	// 转换为字符串，使用正确的精度格式
	priceStr := t.formatFloatWithPrecision(formattedPrice, prec.PricePrecision)
	qtyStr := t.formatFloatWithPrecision(formattedQty, prec.QuantityPrecision)
	log.Printf("  📏 精度处理: 价格 %.8f -> %s (精度=%d), 数量 %.8f -> %s (精度=%d)",
		stopPrice, priceStr, prec.PricePrecision, quantity, qtyStr, prec.QuantityPrecision)

	isIoc := false
	isReduceOnly := true
	isTakeProfit := false
	isStopLoss := true
	order, err := t.PlaceMarketOrder(symbol, prec.TickSize, prec.PairId, priceStr, qtyStr, isLong, isIoc, isReduceOnly, isTakeProfit, isStopLoss)

	log.Printf("stop order create orderID %v", order)
	return err
}

// SetTakeProfit 设置止盈
func (t *ApexTrader) SetTakeProfit(symbol string, positionSide string, quantity, takeProfitPrice float64) error {
	isLong := false

	if positionSide == "SHORT" {
		isLong = true
	}

	// 格式化价格和数量到正确精度
	formattedPrice, err := t.formatPrice(symbol, takeProfitPrice)
	if err != nil {
		return err
	}
	formattedQty, err := t.formatQuantity(symbol, quantity)
	if err != nil {
		return err
	}

	// 获取精度信息
	prec, err := t.getPrecision(symbol)
	if err != nil {
		return err
	}

	// 转换为字符串，使用正确的精度格式
	priceStr := t.formatFloatWithPrecision(formattedPrice, prec.PricePrecision)
	qtyStr := t.formatFloatWithPrecision(formattedQty, prec.QuantityPrecision)

	isIoc := false
	isReduceOnly := true
	isTakeProfit := true
	isStopLoss := false
	order, err := t.PlaceMarketOrder(symbol, prec.TickSize, prec.PairId, priceStr, qtyStr, isLong, isIoc, isReduceOnly, isTakeProfit, isStopLoss)

	log.Printf("stop order create orderID %v", order)
	return err
}

func (t *ApexTrader) GetAllOpenOrders(symbol string) ([]OrderData, error) {

	client := NewApexHttpClient(t.baseURL, t.apikey, t.secret, t.passphrase, WithBaseURL(t.baseURL), WithDebug(true))

	orderResult, err := client.NewUtaApexServiceNoParams().GetOpenOrders(symbol, context.Background())
	if err != nil {
		return nil, err
	}

	if orderResult.Code != 0 {
		return nil, errors.New(orderResult.Message)
	}
	marshalData, _ := json.Marshal(orderResult.Data)
	if marshalData == nil {
		return nil, errors.New("empty data")
	}

	var orderRetData []OrderData
	err = json.Unmarshal([]byte(marshalData), &orderRetData)
	if err != nil {
		return nil, err
	}
	return orderRetData, nil
}

func (t *ApexTrader) CancelOrderByID(id string) (uint64, error) {

	client := NewApexHttpClient(t.baseURL, t.apikey, t.secret, t.passphrase, WithBaseURL(t.baseURL), WithDebug(true))

	params := map[string]interface{}{
		"id": id,
	}
	orderResult, err := client.NewUtaApexServiceWithParams(params).CancelOrderById(context.Background())
	if err != nil {
		return 0, err
	}
	if orderResult.Code != 0 {
		return 0, errors.New(orderResult.Message)
	}

	return strconv.ParseUint(fmt.Sprintf("%v", orderResult.Data), 10, 64)

}

// CancelStopLossOrders 仅取消止损单（不影响止盈单）
func (t *ApexTrader) CancelStopLossOrders(symbol string) error {

	// 获取该币种的所有未完成订单
	orders, err := t.GetAllOpenOrders(symbol)
	if err != nil {
		return err
	}
	// 过滤出止损单并取消（取消所有方向的止损单，包括LONG和SHORT）
	canceledCount := 0
	var cancelErrors []error
	for _, order := range orders {
		if order.Type != stopLimitStr && order.Type != stopMarketStr {
			continue
		}
		orderID, errCode := t.CancelOrderByID(order.ID)
		if errCode != nil {
			cancelErrors = append(cancelErrors, errCode)
		} else {
			log.Printf("success to cancel orderID %v", orderID)
			canceledCount++
		}
	}

	if canceledCount == 0 && len(cancelErrors) == 0 {
		log.Printf("  ℹ %s 没有止损单需要取消", symbol)
	} else if canceledCount > 0 {
		log.Printf("  ✓ 已取消 %s 的 %d 个止损单", symbol, canceledCount)
	}

	// 如果所有取消都失败了，返回错误
	if len(cancelErrors) > 0 && canceledCount == 0 {
		return fmt.Errorf("取消止损单失败: %v", cancelErrors)
	}

	return nil
}

// CancelTakeProfitOrders 仅取消止盈单（不影响止损单）
func (t *ApexTrader) CancelTakeProfitOrders(symbol string) error {

	// 获取该币种的所有未完成订单
	orders, err := t.GetAllOpenOrders(symbol)
	if err != nil {
		return err
	}
	// 过滤出止损单并取消（取消所有方向的止损单，包括LONG和SHORT）
	canceledCount := 0
	var cancelErrors []error
	for _, order := range orders {
		if order.Type != takeProfitLimitStr && order.Type != takeProfitMarketStr {
			continue
		}
		orderID, errCode := t.CancelOrderByID(order.ID)
		if errCode != nil {
			cancelErrors = append(cancelErrors, errCode)
		} else {
			log.Printf("success to cancel orderID %v", orderID)
			canceledCount++
		}
	}

	if canceledCount == 0 && len(cancelErrors) == 0 {
		log.Printf("  ℹ %s 没有止盈单需要取消", symbol)
	} else if canceledCount > 0 {
		log.Printf("  ✓ 已取消 %s 的 %d 个止盈单", symbol, canceledCount)
	}

	// 如果所有取消都失败了，返回错误
	if len(cancelErrors) > 0 && canceledCount == 0 {
		return fmt.Errorf("取消止盈单失败: %v", cancelErrors)
	}

	return nil
}

// CancelAllOrders 取消所有订单
func (t *ApexTrader) CancelAllOrders(symbol string) error {
	client := NewApexHttpClient(t.baseURL, t.apikey, t.secret, t.passphrase, WithBaseURL(t.baseURL), WithDebug(true))

	params := map[string]interface{}{
		"symbol": symbol,
	}
	orderResult, err := client.NewUtaApexServiceWithParams(params).CancelAllOrders(context.Background())
	if err != nil {
		return err
	}
	if orderResult.Code != 0 {
		return errors.New(orderResult.Message)
	}
	return nil
}

// CancelStopOrders 取消该币种的止盈/止损单（用于调整止盈止损位置）
func (t *ApexTrader) CancelStopOrders(symbol string) error {

	// 获取该币种的所有未完成订单
	// 获取该币种的所有未完成订单
	orders, err := t.GetAllOpenOrders(symbol)
	if err != nil {
		return err
	}
	// 过滤出止损单并取消（取消所有方向的止损单，包括LONG和SHORT）
	canceledCount := 0

	for _, order := range orders {
		if order.Type != takeProfitLimitStr && order.Type != takeProfitMarketStr && order.Type != stopLimitStr && order.Type != stopMarketStr {
			continue
		}
		orderID, errCode := t.CancelOrderByID(order.ID)
		if errCode == nil {
			log.Printf("success to cancel orderID %v", orderID)
			canceledCount++
		}
	}

	if canceledCount == 0 {
		log.Printf("  ℹ %s 没有止盈/止损单需要取消", symbol)
	} else {
		log.Printf("  ✓ 已取消 %s 的 %d 个止盈/止损单", symbol, canceledCount)
	}

	return nil
}

// FormatQuantity 格式化数量（实现Trader接口）
func (t *ApexTrader) FormatQuantity(symbol string, quantity float64) (string, error) {
	formatted, err := t.formatQuantity(symbol, quantity)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%v", formatted), nil
}

func (t *ApexTrader) zklinkSign(message string) (string, error) {
	signer, err := zklink.NewZkSignerFromSeeds([]byte(t.zklinkOmniSeeds))
	if err != nil {
		return "", err
	}
	return signer.SignMuSig(message)
}
