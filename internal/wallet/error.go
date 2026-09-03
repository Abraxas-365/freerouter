package wallet

import (
	"net/http"

	"github.com/Abraxas-365/freerouter/internal/errx"
)

var ErrRegistry = errx.NewRegistry("WALLET")

var (
	CodeWalletNotFound      = ErrRegistry.Register("NOT_FOUND", errx.TypeNotFound, http.StatusNotFound, "Wallet not found")
	CodeWalletNameTaken     = ErrRegistry.Register("NAME_TAKEN", errx.TypeConflict, http.StatusConflict, "A wallet with this name already exists")
	CodeInsufficientFunds   = ErrRegistry.Register("INSUFFICIENT_FUNDS", errx.TypeBusiness, http.StatusPaymentRequired, "Insufficient wallet funds")
	CodeInsufficientBalance = ErrRegistry.Register("INSUFFICIENT_BALANCE", errx.TypeBusiness, http.StatusPaymentRequired, "Insufficient main balance to fund wallet")
	CodeWalletNotEmpty      = ErrRegistry.Register("NOT_EMPTY", errx.TypeBusiness, http.StatusConflict, "Wallet must be empty before deletion")
	CodeWalletInUse         = ErrRegistry.Register("IN_USE", errx.TypeBusiness, http.StatusConflict, "Wallet is bound to one or more API keys")
)

func ErrWalletNotFound() *errx.Error      { return ErrRegistry.New(CodeWalletNotFound) }
func ErrWalletNameTaken() *errx.Error     { return ErrRegistry.New(CodeWalletNameTaken) }
func ErrInsufficientFunds() *errx.Error   { return ErrRegistry.New(CodeInsufficientFunds) }
func ErrInsufficientBalance() *errx.Error { return ErrRegistry.New(CodeInsufficientBalance) }
func ErrWalletNotEmpty() *errx.Error      { return ErrRegistry.New(CodeWalletNotEmpty) }
func ErrWalletInUse() *errx.Error         { return ErrRegistry.New(CodeWalletInUse) }
