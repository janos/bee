package mock

import (
	"context"
	"math/big"

	"github.com/ethersphere/bee/pkg/postage/postagecontract"
)

type contractMock struct {
	createBatch func(ctx context.Context, initialBalance *big.Int, depth uint8) ([]byte, error)
}

func (c *contractMock) CreateBatch(ctx context.Context, initialBalance *big.Int, depth uint8) ([]byte, error) {
	return c.createBatch(ctx, initialBalance, depth)
}

// Option is a an option passed to New
type Option func(*contractMock)

// New creates a new mock BatchStore
func New(opts ...Option) postagecontract.Interface {
	bs := &contractMock{}

	for _, o := range opts {
		o(bs)
	}

	return bs
}

func WithCreateBatchFunc(f func(ctx context.Context, initialBalance *big.Int, depth uint8) ([]byte, error)) Option {
	return func(m *contractMock) {
		m.createBatch = f
	}
}
