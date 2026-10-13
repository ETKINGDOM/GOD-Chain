package godrh

import "context"

// ParentNextCommitteeProof proves next_sync_committee in the original retained
// checkpoint state. It is not a provider-selected replacement trust anchor, an
// unbounded update chain, persisted light-client state or activation authority.
type ParentNextCommitteeProof struct {
	Committee       [parentCommitteeSize][48]byte
	AggregatePubkey [48]byte
	Branch          [][32]byte
}

func (ParentNextCommitteeProof) String() string     { return "parent next committee proof (redacted)" }
func (p ParentNextCommitteeProof) GoString() string { return p.String() }
func (ParentNextCommitteeProof) MarshalJSON() ([]byte, error) {
	return []byte(`{"redacted":true}`), nil
}

type ParentCommitteeTransitionProof struct {
	Finality ParentFinalityProof
	Next     ParentNextCommitteeProof
}

func (ParentCommitteeTransitionProof) String() string {
	return "parent adjacent-period proof (redacted)"
}
func (p ParentCommitteeTransitionProof) GoString() string { return p.String() }
func (ParentCommitteeTransitionProof) MarshalJSON() ([]byte, error) {
	return []byte(`{"redacted":true}`), nil
}

type ParentCommitteeTransitionReport struct {
	ParentFinalityReport
	NextCommitteeIncluded                  bool `json:"nextCommitteeIncluded"`
	NextCommitteeAggregateMatched          bool `json:"nextCommitteeAggregateMatched"`
	AdjacentPeriodSignatureVerified        bool `json:"adjacentPeriodSignatureVerified"`
	ConditionalCommitteeTransitionVerified bool `json:"conditionalCommitteeTransitionVerified"`
}

// VerifyParentCommitteeTransitionProof verifies one adjacent-period handoff
// under the original caller-supplied checkpoint/fork/clock trust. The finalized
// header, attestation, signature and current slot must all be in the next
// period. Both committees are included in the original checkpoint state; only
// the proved next committee can authenticate this update. Original checkpoint
// age (at most 8192 slots) and single-fork bounds remain unchanged.
//
// No new root, committee, store mutation, financial admission capability,
// timeout-based finality, transport, signer or broadcast permission is returned.
// Fork transitions, repeated period advancement, independently authenticated
// trust and RH batch/execution/reserve binding remain separate requirements.
func VerifyParentCommitteeTransitionProof(ctx context.Context, t ParentFinalityTrust, p ParentCommitteeTransitionProof) (ParentCommitteeTransitionReport, error) {
	report, err := verifyParentFinalityProof(ctx, t, p.Finality, &p.Next)
	if err != nil {
		return ParentCommitteeTransitionReport{}, err
	}
	return ParentCommitteeTransitionReport{
		ParentFinalityReport: report, NextCommitteeIncluded: true,
		NextCommitteeAggregateMatched: true, AdjacentPeriodSignatureVerified: true,
		ConditionalCommitteeTransitionVerified: true,
	}, nil
}
