package msg

import (
	"bytes"
	"context"
	"math"

	"github.com/ETKINGDOM/GOD-Chain/x/godbridge"
	rewardmsg "github.com/ETKINGDOM/GOD-Chain/x/godrewards/msg"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/msgservice"
)

func RegisterInterfaces(registry codectypes.InterfaceRegistry) {
	registry.RegisterImplementations((*sdk.Msg)(nil), &MsgAcceptDeposit{}, &MsgRequestWithdrawal{}, &MsgAuthorizeWithdrawal{},
		&MsgResolvePayment{}, &MsgCancelWithdrawal{}, &MsgPauseBridge{})
	msgservice.RegisterMsgServiceDesc(registry, &_Msg_serviceDesc)
}

type server struct {
	keeper      godbridge.Keeper
	approvalGas uint64
}

// NewServer selects an explicit prototype gas cost for each quorum signature.
// There is no mainnet default. Only mount behind the reviewed native ante;
// outer sender authentication is not repeated by direct keeper/server calls.
func NewServer(keeper godbridge.Keeper, approvalGas uint64) (MsgServer, error) {
	if !keeper.IsConfigured() || approvalGas == 0 || approvalGas > math.MaxInt64/godbridge.SignerCount {
		return nil, ErrMessage
	}
	return server{keeper: keeper, approvalGas: approvalGas}, nil
}

func (s server) charge(ctx sdk.Context, signatures [][]byte) {
	ctx.GasMeter().ConsumeGas(uint64(len(signatures))*s.approvalGas, "God bridge quorum verification")
}

func (s server) matched(ctx sdk.Context, sequence uint64, id []byte) error {
	w, err := s.keeper.Withdrawal(ctx, sequence)
	if err != nil {
		return err
	}
	if !bytes.Equal(w.ID[:], id) {
		return godbridge.ErrRequest
	}
	return nil
}

func (s server) AcceptDeposit(ctx context.Context, m *MsgAcceptDeposit) (*MsgAcceptDepositResponse, error) {
	if err := Validate(m); err != nil {
		return nil, err
	}
	c := sdk.UnwrapSDKContext(ctx)
	s.charge(c, m.Approvals)
	recipient, _ := rewardmsg.Account(m.Recipient)
	n, _ := amount(m.Amount)
	e, _ := evidence(m.Evidence)
	if err := s.keeper.AcceptDeposit(c, godbridge.Deposit{Sequence: m.DepositSequence, Recipient: [20]byte(recipient), Amount: n, Evidence: e}, m.Approvals); err != nil {
		return nil, err
	}
	return &MsgAcceptDepositResponse{}, nil
}

func (s server) RequestWithdrawal(ctx context.Context, m *MsgRequestWithdrawal) (*MsgRequestWithdrawalResponse, error) {
	if err := Validate(m); err != nil {
		return nil, err
	}
	owner, _ := rewardmsg.Account(m.Sender)
	n, _ := amount(m.Amount)
	w, err := s.keeper.RequestWithdrawal(sdk.UnwrapSDKContext(ctx), owner, [20]byte(m.SourceRecipient), n)
	if err != nil {
		return nil, err
	}
	return &MsgRequestWithdrawalResponse{WithdrawalSequence: w.Sequence, WithdrawalId: append([]byte(nil), w.ID[:]...)}, nil
}

func (s server) AuthorizeWithdrawal(ctx context.Context, m *MsgAuthorizeWithdrawal) (*MsgAuthorizeWithdrawalResponse, error) {
	if err := Validate(m); err != nil {
		return nil, err
	}
	c := sdk.UnwrapSDKContext(ctx)
	if err := s.matched(c, m.WithdrawalSequence, m.WithdrawalId); err != nil {
		return nil, err
	}
	s.charge(c, m.Approvals)
	if err := s.keeper.AuthorizeNext(c, m.WithdrawalSequence, m.Approvals); err != nil {
		return nil, err
	}
	return &MsgAuthorizeWithdrawalResponse{}, nil
}

func (s server) ResolvePayment(ctx context.Context, m *MsgResolvePayment) (*MsgResolvePaymentResponse, error) {
	if err := Validate(m); err != nil {
		return nil, err
	}
	c := sdk.UnwrapSDKContext(ctx)
	if err := s.matched(c, m.WithdrawalSequence, m.WithdrawalId); err != nil {
		return nil, err
	}
	s.charge(c, m.Approvals)
	e, _ := evidence(m.Evidence)
	if err := s.keeper.ResolvePayment(c, m.WithdrawalSequence, e, m.Approvals); err != nil {
		return nil, err
	}
	return &MsgResolvePaymentResponse{}, nil
}

func (s server) CancelWithdrawal(ctx context.Context, m *MsgCancelWithdrawal) (*MsgCancelWithdrawalResponse, error) {
	if err := Validate(m); err != nil {
		return nil, err
	}
	c := sdk.UnwrapSDKContext(ctx)
	if err := s.matched(c, m.WithdrawalSequence, m.WithdrawalId); err != nil {
		return nil, err
	}
	s.charge(c, m.Approvals)
	e, _ := evidence(m.Evidence)
	if err := s.keeper.CancelWithdrawal(c, m.WithdrawalSequence, e, m.Approvals); err != nil {
		return nil, err
	}
	return &MsgCancelWithdrawalResponse{}, nil
}

func (s server) PauseBridge(ctx context.Context, m *MsgPauseBridge) (*MsgPauseBridgeResponse, error) {
	if err := Validate(m); err != nil {
		return nil, err
	}
	c := sdk.UnwrapSDKContext(ctx)
	s.charge(c, m.Approvals)
	if err := s.keeper.Pause(c, m.ControlNonce, m.Intake, m.Outflow, m.Approvals); err != nil {
		return nil, err
	}
	return &MsgPauseBridgeResponse{}, nil
}
