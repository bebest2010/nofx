package zklink

import (
	"crypto/sha256"
	"fmt"
	"math/big"

	zksdk "nofx/libs/zklink_sdk"

	"github.com/shopspring/decimal"
)

type ZkSigner interface {
	PublicKey() string
	SignMuSig(msgNo0x string) (string, error)
	SignOrder(accountIdOrig, clientId, symbol, size, price string, pairID uint32, isBuy bool, takeFeeRate, makeFeeRate string) (string, error)
}

func NewZkSignerFromSeeds(seeds []byte) (ZkSigner, error) {
	s, err := zksdk.ZkLinkSignerNewFromSeed(seeds)
	if err != nil {
		return nil, fmt.Errorf("NewZkLinkSignerFromSeed failed: %w", err)
	}
	return &signerImpl{inner: s}, nil
}

type signerImpl struct{ inner *zksdk.ZkLinkSigner }

func (s *signerImpl) PublicKey() string { return s.inner.PublicKey() }

func (s *signerImpl) SignMuSig(msgNo0x string) (string, error) {
	out, err := s.inner.SignMusig([]byte(msgNo0x))
	if err != nil {
		return "", err
	}
	if out.Signature != "" {
		return out.Signature, nil
	}
	return "", fmt.Errorf("empty MuSig signature")
}

func getNonceFromClientId(clientID string) (int64, int64) {
	hash := sha256.Sum256([]byte(clientID))

	// 将哈希字节转换为大整数
	dataBn := new(big.Int).SetBytes(hash[:])

	slotID := big.NewInt(0).Mod(dataBn, big.NewInt(0).SetUint64(1<<64-1))
	slotIDRes := big.NewInt(0).Div(slotID, big.NewInt(1<<32-1))
	nonce := big.NewInt(0).Mod(dataBn, big.NewInt(1<<32-1))
	return slotIDRes.Int64(), nonce.Int64()
}
func (s *signerImpl) SignOrder(accountIdOrig, clientId, symbol, size, price string, pairID uint32, isBuy bool, takeFeeRate, makeFeeRate string) (string, error) {

	accountIDInt, _ := big.NewInt(0).SetString(accountIdOrig, 10)
	accountId := big.NewInt(0).Mod(accountIDInt, big.NewInt(1<<32-1))

	sizeDec, _ := decimal.NewFromString(size)
	sizeDecBig := sizeDec.Mul(decimal.New(1, 18))
	priceDec, _ := decimal.NewFromString(price)
	priceDecBig := priceDec.Mul(decimal.New(1, 18))
	takeFeeRateDec, _ := decimal.NewFromString(takeFeeRate)
	takeFeeRateDecBig := takeFeeRateDec.Mul(decimal.New(1, 4))
	makeFeeRateDec, _ := decimal.NewFromString(makeFeeRate)
	makeFeeRateDecBig := makeFeeRateDec.Mul(decimal.New(1, 4))
	slot, nonce := getNonceFromClientId(clientId)
	contractBuilder := zksdk.ContractBuilder{
		zksdk.AccountId(accountId.Int64()),
		zksdk.SubAccountId(0),
		zksdk.SlotId(slot),
		zksdk.Nonce(nonce),
		zksdk.PairId(pairID),
		*sizeDecBig.BigInt(),
		*priceDecBig.BigInt(),
		isBuy,
		uint8(takeFeeRateDecBig.IntPart()),
		uint8(makeFeeRateDecBig.IntPart()),
		false,
	}
	order := zksdk.NewContract(contractBuilder)
	out, err := order.CreateSignedContract(s.inner)
	if err != nil {
		return "", err
	}
	signature := out.GetSignature()

	return signature.Signature, nil

}
