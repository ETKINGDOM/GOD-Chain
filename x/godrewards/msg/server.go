package msg

import (
	"context"
	"time"

	"github.com/ETKINGDOM/GOD-Chain/x/godrewards"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/msgservice"
)

func RegisterInterfaces(registry codectypes.InterfaceRegistry) {
	registry.RegisterImplementations((*sdk.Msg)(nil),
		&MsgClaimG{}, &MsgTransferG{}, &MsgLockG{}, &MsgUnlockG{}, &MsgRedeemG{}, &MsgDonateGod{})
	msgservice.RegisterMsgServiceDesc(registry, &_Msg_serviceDesc)
}

type server struct{ keeper godrewards.Keeper }

// NewServer is a trusted transaction-router adapter, not an authentication
// boundary. The application must attach the reviewed ante chain to BaseApp.
func NewServer(keeper godrewards.Keeper) MsgServer { return server{keeper} }

func (s server) ClaimG(ctx context.Context, m *MsgClaimG) (*MsgClaimGResponse, error) {
	if err := Validate(m); err != nil {
		return nil, err
	}
	owner, _ := Account(m.Sender)
	if err := s.keeper.Claim(sdk.UnwrapSDKContext(ctx), owner); err != nil {
		return nil, err
	}
	return &MsgClaimGResponse{}, nil
}

func (s server) TransferG(ctx context.Context, m *MsgTransferG) (*MsgTransferGResponse, error) {
	if err := Validate(m); err != nil {
		return nil, err
	}
	owner, _ := Account(m.Sender)
	recipient, _ := Account(m.Recipient)
	amount, _ := Amount(m.Amount)
	if err := s.keeper.TransferG(sdk.UnwrapSDKContext(ctx), owner, recipient, amount); err != nil {
		return nil, err
	}
	return &MsgTransferGResponse{}, nil
}

func (s server) LockG(ctx context.Context, m *MsgLockG) (*MsgLockGResponse, error) {
	if err := Validate(m); err != nil {
		return nil, err
	}
	owner, _ := Account(m.Sender)
	amount, _ := Amount(m.Amount)
	if err := s.keeper.LockG(sdk.UnwrapSDKContext(ctx), owner, m.LockId, amount); err != nil {
		return nil, err
	}
	return &MsgLockGResponse{}, nil
}

func (s server) UnlockG(ctx context.Context, m *MsgUnlockG) (*MsgUnlockGResponse, error) {
	if err := Validate(m); err != nil {
		return nil, err
	}
	owner, _ := Account(m.Sender)
	if err := s.keeper.UnlockG(sdk.UnwrapSDKContext(ctx), owner, m.LockId); err != nil {
		return nil, err
	}
	return &MsgUnlockGResponse{}, nil
}

func (s server) RedeemG(ctx context.Context, m *MsgRedeemG) (*MsgRedeemGResponse, error) {
	if err := Validate(m); err != nil {
		return nil, err
	}
	owner, _ := Account(m.Sender)
	beneficiary, _ := Account(m.Beneficiary)
	amount, _ := Amount(m.Amount)
	minimum, _ := Amount(m.MinGodOut)
	paid, err := s.keeper.Redeem(sdk.UnwrapSDKContext(ctx), godrewards.Redemption{
		Sender: owner, Beneficiary: beneficiary, G: amount, MinGodOut: minimum,
		Deadline: time.Unix(0, m.DeadlineUnixNanos).UTC(),
	})
	if err != nil {
		return nil, err
	}
	return &MsgRedeemGResponse{GodOut: paid.String()}, nil
}

func (s server) DonateGod(ctx context.Context, m *MsgDonateGod) (*MsgDonateGodResponse, error) {
	if err := Validate(m); err != nil {
		return nil, err
	}
	owner, _ := Account(m.Sender)
	amount, _ := Amount(m.Amount)
	if err := s.keeper.Donate(sdk.UnwrapSDKContext(ctx), owner, amount); err != nil {
		return nil, err
	}
	return &MsgDonateGodResponse{}, nil
}
