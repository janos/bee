// Copyright 2020 The Swarm Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package chequebook_test

import (
	"context"
	"errors"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethersphere/bee/pkg/settlement/swap/chequebook"
	storemock "github.com/ethersphere/bee/pkg/statestore/mock"
	"github.com/ethersphere/bee/pkg/storage"
)

func newTestChequebook(
	t *testing.T,
	backend chequebook.Backend,
	transactionService chequebook.TransactionService,
	address,
	erc20address,
	ownerAdress common.Address,
	store storage.StateStorer,
	chequeSigner chequebook.ChequeSigner,
	simpleSwapBinding chequebook.SimpleSwapBinding,
	erc20Binding chequebook.ERC20Binding) (chequebook.Service, error) {
	return chequebook.New(
		backend,
		transactionService,
		address,
		erc20address,
		ownerAdress,
		store,
		chequeSigner,
		func(addr common.Address, b bind.ContractBackend) (chequebook.SimpleSwapBinding, error) {
			if addr != address {
				t.Fatalf("initialised binding with wrong address. wanted %x, got %x", address, addr)
			}
			if b != backend {
				t.Fatal("initialised binding with wrong backend")
			}
			return simpleSwapBinding, nil
		},
		func(addr common.Address, b bind.ContractBackend) (chequebook.ERC20Binding, error) {
			if addr != erc20address {
				t.Fatalf("initialised binding with wrong address. wanted %x, got %x", erc20address, addr)
			}
			if b != backend {
				t.Fatal("initialised binding with wrong backend")
			}
			return erc20Binding, nil
		},
	)
}

func TestChequebookAddress(t *testing.T) {
	address := common.HexToAddress("0xabcd")
	erc20address := common.HexToAddress("0xefff")
	ownerAdress := common.HexToAddress("0xfff")
	chequebookService, err := newTestChequebook(
		t,
		&backendMock{},
		&transactionServiceMock{},
		address,
		erc20address,
		ownerAdress,
		nil,
		&chequeSignerMock{},
		&simpleSwapBindingMock{},
		&erc20BindingMock{})
	if err != nil {
		t.Fatal(err)
	}

	if chequebookService.Address() != address {
		t.Fatalf("returned wrong address. wanted %x, got %x", address, chequebookService.Address())
	}
}

func TestChequebookBalance(t *testing.T) {
	address := common.HexToAddress("0xabcd")
	erc20address := common.HexToAddress("0xefff")
	ownerAdress := common.HexToAddress("0xfff")
	balance := big.NewInt(10)
	chequebookService, err := newTestChequebook(
		t,
		&backendMock{},
		&transactionServiceMock{},
		address,
		erc20address,
		ownerAdress,
		nil,
		&chequeSignerMock{},
		&simpleSwapBindingMock{
			balance: func(*bind.CallOpts) (*big.Int, error) {
				return balance, nil
			},
		},
		&erc20BindingMock{})
	if err != nil {
		t.Fatal(err)
	}

	returnedBalance, err := chequebookService.Balance(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	if returnedBalance.Cmp(balance) != 0 {
		t.Fatalf("returned wrong balance. wanted %d, got %d", balance, returnedBalance)
	}
}

func TestChequebookDeposit(t *testing.T) {
	address := common.HexToAddress("0xabcd")
	erc20address := common.HexToAddress("0xefff")
	ownerAdress := common.HexToAddress("0xfff")
	balance := big.NewInt(30)
	depositAmount := big.NewInt(20)
	txHash := common.HexToHash("0xdddd")
	chequebookService, err := newTestChequebook(
		t,
		&backendMock{},
		&transactionServiceMock{
			send: func(c context.Context, request *chequebook.TxRequest) (common.Hash, error) {
				if request.To != erc20address {
					t.Fatalf("sending to wrong contract. wanted %x, got %x", erc20address, request.To)
				}
				if request.Value.Cmp(big.NewInt(0)) != 0 {
					t.Fatal("sending ether to token contract")
				}
				return txHash, nil
			},
		},
		address,
		erc20address,
		ownerAdress,
		nil,
		&chequeSignerMock{},
		&simpleSwapBindingMock{},
		&erc20BindingMock{
			balanceOf: func(b *bind.CallOpts, addr common.Address) (*big.Int, error) {
				if addr != ownerAdress {
					t.Fatalf("looking up balance of wrong account. wanted %x, got %x", ownerAdress, addr)
				}
				return balance, nil
			},
		})
	if err != nil {
		t.Fatal(err)
	}

	returnedTxHash, err := chequebookService.Deposit(context.Background(), depositAmount)
	if err != nil {
		t.Fatal(err)
	}

	if txHash != returnedTxHash {
		t.Fatalf("returned wrong transaction hash. wanted %v, got %v", txHash, returnedTxHash)
	}
}

func TestChequebookWaitForDeposit(t *testing.T) {
	address := common.HexToAddress("0xabcd")
	erc20address := common.HexToAddress("0xefff")
	ownerAdress := common.HexToAddress("0xfff")
	txHash := common.HexToHash("0xdddd")
	chequebookService, err := newTestChequebook(
		t,
		&backendMock{},
		&transactionServiceMock{
			waitForReceipt: func(ctx context.Context, tx common.Hash) (*types.Receipt, error) {
				if tx != txHash {
					t.Fatalf("waiting for wrong transaction. wanted %x, got %x", txHash, tx)
				}
				return &types.Receipt{
					Status: 1,
				}, nil
			},
		},
		address,
		erc20address,
		ownerAdress,
		nil,
		&chequeSignerMock{},
		&simpleSwapBindingMock{},
		&erc20BindingMock{})
	if err != nil {
		t.Fatal(err)
	}

	err = chequebookService.WaitForDeposit(context.Background(), txHash)
	if err != nil {
		t.Fatal(err)
	}
}

func TestChequebookWaitForDepositReverted(t *testing.T) {
	address := common.HexToAddress("0xabcd")
	erc20address := common.HexToAddress("0xefff")
	ownerAdress := common.HexToAddress("0xfff")
	txHash := common.HexToHash("0xdddd")
	chequebookService, err := newTestChequebook(
		t,
		&backendMock{},
		&transactionServiceMock{
			waitForReceipt: func(ctx context.Context, tx common.Hash) (*types.Receipt, error) {
				if tx != txHash {
					t.Fatalf("waiting for wrong transaction. wanted %x, got %x", txHash, tx)
				}
				return &types.Receipt{
					Status: 0,
				}, nil
			},
		},
		address,
		erc20address,
		ownerAdress,
		nil,
		&chequeSignerMock{},
		&simpleSwapBindingMock{},
		&erc20BindingMock{})
	if err != nil {
		t.Fatal(err)
	}

	err = chequebookService.WaitForDeposit(context.Background(), txHash)
	if err == nil {
		t.Fatal("expected reverted error")
	}
	if !errors.Is(err, chequebook.ErrTransactionReverted) {
		t.Fatalf("wrong error. wanted %v, got %v", chequebook.ErrTransactionReverted, err)
	}
}

func TestChequebookIssue(t *testing.T) {
	address := common.HexToAddress("0xabcd")
	erc20address := common.HexToAddress("0xefff")
	beneficiary := common.HexToAddress("0xdddd")
	ownerAdress := common.HexToAddress("0xfff")
	store := storemock.NewStateStore()
	amount := big.NewInt(20)
	amount2 := big.NewInt(30)
	expectedCumulative := big.NewInt(50)
	sig := common.Hex2Bytes("0xffff")
	chequeSigner := &chequeSignerMock{}

	chequebookService, err := newTestChequebook(
		t,
		&backendMock{},
		&transactionServiceMock{},
		address,
		erc20address,
		ownerAdress,
		store,
		chequeSigner,
		&simpleSwapBindingMock{},
		&erc20BindingMock{})
	if err != nil {
		t.Fatal(err)
	}

	// issue a cheque
	expectedCheque := &chequebook.SignedCheque{
		Cheque: chequebook.Cheque{
			Beneficiary:      beneficiary,
			CumulativePayout: amount,
			Chequebook:       address,
		},
		Signature: sig,
	}

	chequeSigner.sign = func(cheque *chequebook.Cheque) ([]byte, error) {
		if !cheque.Equal(&expectedCheque.Cheque) {
			t.Fatalf("wrong cheque. wanted %v got %v", expectedCheque.Cheque, cheque)
		}
		return sig, nil
	}

	err = chequebookService.Issue(beneficiary, amount, func(cheque *chequebook.SignedCheque) error {
		if !cheque.Equal(expectedCheque) {
			t.Fatalf("wrong cheque. wanted %v got %v", expectedCheque, cheque)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	lastCheque, err := chequebookService.LastCheque(beneficiary)
	if err != nil {
		t.Fatal(err)
	}

	if !lastCheque.Equal(expectedCheque) {
		t.Fatalf("wrong cheque stored. wanted %v got %v", expectedCheque, lastCheque)
	}

	// issue another cheque for the same beneficiary
	expectedCheque = &chequebook.SignedCheque{
		Cheque: chequebook.Cheque{
			Beneficiary:      beneficiary,
			CumulativePayout: expectedCumulative,
			Chequebook:       address,
		},
		Signature: sig,
	}

	chequeSigner.sign = func(cheque *chequebook.Cheque) ([]byte, error) {
		if !cheque.Equal(&expectedCheque.Cheque) {
			t.Fatalf("wrong cheque. wanted %v got %v", expectedCheque, cheque)
		}
		return sig, nil
	}

	err = chequebookService.Issue(beneficiary, amount2, func(cheque *chequebook.SignedCheque) error {
		if !cheque.Equal(expectedCheque) {
			t.Fatalf("wrong cheque. wanted %v got %v", expectedCheque, cheque)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	lastCheque, err = chequebookService.LastCheque(beneficiary)
	if err != nil {
		t.Fatal(err)
	}

	if !lastCheque.Equal(expectedCheque) {
		t.Fatalf("wrong cheque stored. wanted %v got %v", expectedCheque, lastCheque)
	}

	// issue another cheque for the different beneficiary
	expectedChequeOwner := &chequebook.SignedCheque{
		Cheque: chequebook.Cheque{
			Beneficiary:      ownerAdress,
			CumulativePayout: amount,
			Chequebook:       address,
		},
		Signature: sig,
	}

	chequeSigner.sign = func(cheque *chequebook.Cheque) ([]byte, error) {
		if !cheque.Equal(&expectedChequeOwner.Cheque) {
			t.Fatalf("wrong cheque. wanted %v got %v", expectedCheque, cheque)
		}
		return sig, nil
	}

	err = chequebookService.Issue(ownerAdress, amount, func(cheque *chequebook.SignedCheque) error {
		if !cheque.Equal(expectedChequeOwner) {
			t.Fatalf("wrong cheque. wanted %v got %v", expectedChequeOwner, cheque)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	lastCheque, err = chequebookService.LastCheque(ownerAdress)
	if err != nil {
		t.Fatal(err)
	}

	if !lastCheque.Equal(expectedChequeOwner) {
		t.Fatalf("wrong cheque stored. wanted %v got %v", expectedChequeOwner, lastCheque)
	}

	// finally check this did not interfere with the beneficiary cheque
	lastCheque, err = chequebookService.LastCheque(beneficiary)
	if err != nil {
		t.Fatal(err)
	}

	if !lastCheque.Equal(expectedCheque) {
		t.Fatalf("wrong cheque stored. wanted %v got %v", expectedCheque, lastCheque)
	}
}

func TestChequebookIssueFailedSend(t *testing.T) {
	address := common.HexToAddress("0xabcd")
	erc20address := common.HexToAddress("0xefff")
	beneficiary := common.HexToAddress("0xdddd")
	ownerAdress := common.HexToAddress("0xfff")
	store := storemock.NewStateStore()
	amount := big.NewInt(20)
	sig := common.Hex2Bytes("0xffff")
	chequeSigner := &chequeSignerMock{}

	chequebookService, err := newTestChequebook(
		t,
		&backendMock{},
		&transactionServiceMock{},
		address,
		erc20address,
		ownerAdress,
		store,
		chequeSigner,
		&simpleSwapBindingMock{},
		&erc20BindingMock{})
	if err != nil {
		t.Fatal(err)
	}

	chequeSigner.sign = func(cheque *chequebook.Cheque) ([]byte, error) {
		return sig, nil
	}

	err = chequebookService.Issue(beneficiary, amount, func(cheque *chequebook.SignedCheque) error {
		return errors.New("err")
	})
	if err == nil {
		t.Fatal("expected error")
	}

	// verify the cheque was not saved
	_, err = chequebookService.LastCheque(beneficiary)
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, chequebook.ErrNoCheque) {
		t.Fatalf("wrong error. wanted %v, got %v", chequebook.ErrNoCheque, err)
	}
}
