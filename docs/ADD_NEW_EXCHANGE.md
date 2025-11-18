# 添加新交易所支持指南

本文档详细说明如何在 NOFX 系统中添加新的交易所支持。

## 📋 需要修改的文件清单

### 1. 后端核心文件

#### 1.1 实现交易所接口 (`trader/`)
- **创建新文件**: `trader/{exchange_name}_trader.go`
  - 实现 `trader.Trader` 接口的所有方法
  - 参考: `binance_futures.go`, `hyperliquid_trader.go`, `aster_trader.go`
  
- **创建测试文件**: `trader/{exchange_name}_trader_test.go`
  - 编写单元测试确保功能正常

#### 1.2 更新交易器配置 (`trader/auto_trader.go`)
- **位置**: `trader/auto_trader.go` 第 18-40 行
- **修改**: 在 `AutoTraderConfig` 结构体中添加新交易所的配置字段
```go
// 新交易所配置
NewExchangeAPIKey    string
NewExchangeSecretKey  string
NewExchangeTestnet    bool  // 如果需要测试网支持
```

- **位置**: `trader/auto_trader.go` 第 177-195 行
- **修改**: 在 `switch config.Exchange` 中添加新分支
```go
case "new_exchange":
    log.Printf("🏦 [%s] 使用新交易所交易", config.Name)
    trader, err = NewNewExchangeTrader(
        config.NewExchangeAPIKey,
        config.NewExchangeSecretKey,
        config.NewExchangeTestnet,
    )
    if err != nil {
        return nil, fmt.Errorf("初始化新交易所交易器失败: %w", err)
    }
```

#### 1.3 更新数据库模型 (`config/database.go`)
- **位置**: `config/database.go` 第 888 行
- **修改**: `CreateExchange` 函数签名，添加新交易所的字段
```go
func (d *Database) CreateExchange(
    userID, id, name, typ string, 
    enabled bool, 
    apiKey, secretKey string, 
    testnet bool, 
    hyperliquidWalletAddr, 
    asterUser, asterSigner, asterPrivateKey string,
    newExchangeAPIKey, newExchangeSecretKey string, // 新增
) error
```

- **位置**: `config/database.go` 第 860-864 行
- **修改**: `UpdateExchange` 函数中的 SQL INSERT 语句，添加新字段
```sql
INSERT INTO exchanges (..., new_exchange_api_key, new_exchange_secret_key)
VALUES (..., ?, ?)
```

- **位置**: `config/database.go` 第 820-830 行
- **修改**: `GetExchanges` 函数中的 SQL SELECT 语句，添加新字段
```sql
SELECT ..., new_exchange_api_key, new_exchange_secret_key
FROM exchanges
```

- **位置**: `config/database.go` 第 700-750 行（ExchangeConfig 结构体）
- **修改**: 添加新字段
```go
type ExchangeConfig struct {
    // ... 现有字段
    NewExchangeAPIKey    string `json:"new_exchange_api_key"`
    NewExchangeSecretKey  string `json:"new_exchange_secret_key"`
}
```

- **位置**: 数据库迁移
- **修改**: 在数据库初始化时添加新字段的迁移逻辑

#### 1.4 更新交易管理器 (`manager/trader_manager.go`)
- **位置**: `manager/trader_manager.go` 第 245-255 行
- **修改**: 在 `loadTraderFromDB` 函数中添加新交易所的配置读取
```go
} else if exchangeCfg.ID == "new_exchange" {
    traderConfig.NewExchangeAPIKey = exchangeCfg.NewExchangeAPIKey
    traderConfig.NewExchangeSecretKey = exchangeCfg.NewExchangeSecretKey
    traderConfig.NewExchangeTestnet = exchangeCfg.Testnet
}
```

- **位置**: `manager/trader_manager.go` 第 351-360 行
- **修改**: 在 `AddTraderFromDB` 函数中添加相同逻辑

- **位置**: `manager/trader_manager.go` 第 1048-1061 行
- **修改**: 在 `StartTrader` 函数中添加相同逻辑

#### 1.5 更新 API 服务器 (`api/server.go`)
- **位置**: `api/server.go` 第 566-583 行
- **修改**: 在创建临时 trader 的 switch 语句中添加新分支
```go
case "new_exchange":
    tempTrader, createErr = trader.NewNewExchangeTrader(
        exchangeCfg.NewExchangeAPIKey,
        exchangeCfg.NewExchangeSecretKey,
        exchangeCfg.Testnet,
    )
```

- **位置**: `api/server.go` 第 200-300 行（SafeExchangeConfig 结构体）
- **修改**: 添加新字段到安全响应结构
```go
type SafeExchangeConfig struct {
    // ... 现有字段
    NewExchangeAPIKey    string `json:"new_exchange_api_key,omitempty"`
}
```

- **位置**: `api/server.go` 第 400-500 行（ExchangeConfigRequest 结构体）
- **修改**: 添加新字段到请求结构
```go
type ExchangeConfigRequest struct {
    // ... 现有字段
    NewExchangeAPIKey    string `json:"new_exchange_api_key"`
    NewExchangeSecretKey string `json:"new_exchange_secret_key"`
}
```

### 2. 前端文件

#### 2.1 交易所图标 (`web/src/components/ExchangeIcons.tsx`)
- **位置**: `web/src/components/ExchangeIcons.tsx`
- **修改**: 
  1. 添加新交易所的 SVG 图标组件
  2. 在 `getExchangeIcon` 函数的 switch 语句中添加新分支

```tsx
// 添加图标组件
const NewExchangeIcon: React.FC<IconProps> = ({ ... }) => (
  <svg>...</svg>
)

// 在 getExchangeIcon 中添加
case 'new_exchange':
  return <NewExchangeIcon {...iconProps} />
```

#### 2.2 交易所配置模态框 (`web/src/components/traders/ExchangeConfigModal.tsx`)
- **位置**: `web/src/components/traders/ExchangeConfigModal.tsx`
- **修改**:
  1. 添加状态变量存储新交易所的配置
  2. 在表单中添加新交易所的输入字段
  3. 在提交逻辑中处理新交易所的配置

```tsx
const [newExchangeAPIKey, setNewExchangeAPIKey] = useState('')
const [newExchangeSecretKey, setNewExchangeSecretKey] = useState('')

// 在表单中添加
{selectedExchange?.id === 'new_exchange' && (
  <>
    <input 
      value={newExchangeAPIKey}
      onChange={(e) => setNewExchangeAPIKey(e.target.value)}
      placeholder="API Key"
    />
    <input 
      value={newExchangeSecretKey}
      onChange={(e) => setNewExchangeSecretKey(e.target.value)}
      placeholder="Secret Key"
    />
  </>
)}
```

#### 2.3 AI 交易员页面 (`web/src/components/AITradersPage.tsx`)
- **位置**: `web/src/components/AITradersPage.tsx`
- **修改**: 与 ExchangeConfigModal 类似的修改，添加新交易所的配置处理

#### 2.4 国际化翻译 (`web/src/i18n/translations.ts`)
- **位置**: `web/src/i18n/translations.ts`
- **修改**: 添加新交易所的名称翻译
```ts
newExchangeName: 'New Exchange',
// 中文
newExchangeName: '新交易所',
```

#### 2.5 交易员操作 Hook (`web/src/hooks/useTraderActions.ts`)
- **位置**: `web/src/hooks/useTraderActions.ts` 第 470-473 行和 559-562 行
- **修改**: 在创建/更新交易员时添加新交易所的字段
```ts
new_exchange_api_key: exchange.newExchangeAPIKey || '',
new_exchange_secret_key: exchange.newExchangeSecretKey || '',
```

### 3. 数据库迁移

#### 3.1 数据库 Schema 更新
- **位置**: `config/database.go` 中的数据库初始化代码
- **修改**: 在 `exchanges` 表的创建/迁移语句中添加新字段
```sql
ALTER TABLE exchanges ADD COLUMN new_exchange_api_key TEXT;
ALTER TABLE exchanges ADD COLUMN new_exchange_secret_key TEXT;
```

### 4. 文档更新

#### 4.1 README 文档
- **位置**: `README.md`
- **修改**: 在支持的交易所列表中添加新交易所

#### 4.2 国际化 README
- **位置**: `docs/i18n/*/README.md`
- **修改**: 更新所有语言的 README 文档

## 🔧 实现步骤

### 步骤 1: 实现 Trader 接口
1. 创建 `trader/{exchange_name}_trader.go`
2. 实现所有必需的方法：
   - `GetBalance()`
   - `GetPositions()`
   - `OpenLong()` / `OpenShort()`
   - `CloseLong()` / `CloseShort()`
   - `SetLeverage()`
   - `SetMarginMode()`
   - `GetMarketPrice()`
   - `SetStopLoss()` / `SetTakeProfit()`
   - `CancelStopLossOrders()` / `CancelTakeProfitOrders()`
   - `CancelAllOrders()` / `CancelStopOrders()`
   - `FormatQuantity()`

### 步骤 2: 更新配置结构
1. 在 `AutoTraderConfig` 中添加新字段
2. 在 `ExchangeConfig` 中添加新字段
3. 更新数据库模型

### 步骤 3: 集成到系统
1. 在 `auto_trader.go` 中添加 switch case
2. 在 `trader_manager.go` 中添加配置读取逻辑
3. 在 `api/server.go` 中添加 API 处理逻辑

### 步骤 4: 前端集成
1. 添加交易所图标
2. 更新配置表单
3. 更新国际化翻译

### 步骤 5: 测试
1. 编写单元测试
2. 测试创建交易员
3. 测试交易功能
4. 测试前端界面

## 📝 注意事项

1. **接口一致性**: 确保新交易所实现完全符合 `Trader` 接口规范
2. **错误处理**: 妥善处理 API 调用失败、网络错误等情况
3. **精度处理**: 注意不同交易所的数量和价格精度要求
4. **杠杆限制**: 不同交易所的杠杆限制可能不同
5. **测试网支持**: 如果交易所支持测试网，添加 `testnet` 参数
6. **安全性**: 敏感信息（API Key、Secret Key）需要加密存储
7. **缓存策略**: 考虑实现余额和持仓的缓存机制以提高性能

## 🔍 参考实现

- **Binance**: `trader/binance_futures.go` - CEX 实现示例
- **Hyperliquid**: `trader/hyperliquid_trader.go` - DEX 实现示例
- **Aster**: `trader/aster_trader.go` - DEX 实现示例

## ✅ 检查清单

- [ ] 实现 `Trader` 接口的所有方法
- [ ] 添加单元测试
- [ ] 更新 `AutoTraderConfig` 结构体
- [ ] 更新 `ExchangeConfig` 结构体
- [ ] 更新数据库 Schema
- [ ] 在 `auto_trader.go` 中添加 switch case
- [ ] 在 `trader_manager.go` 中添加配置读取
- [ ] 在 `api/server.go` 中添加 API 处理
- [ ] 添加前端图标组件
- [ ] 更新配置表单
- [ ] 更新国际化翻译
- [ ] 更新文档
- [ ] 测试所有功能



CGO_LDFLAGS="-lzklink_sdk -L/Users/dlmbp015/go/src/github.com/bebest2010/nofx/libs/mac_arm64_release -lm -ldl"  CGO_ENABLED=1 go build