package zklink

import (
	"fmt"
	zksdk "nofx/libs/zklink_sdk"
)

type ZkSigner interface {
	PublicKey() string
	SignMuSig(msgNo0x string) (string, error)
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
