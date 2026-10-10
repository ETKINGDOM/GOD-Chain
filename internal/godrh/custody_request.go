package godrh

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"io"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	sdkmath "cosmossdk.io/math"
	"github.com/ETKINGDOM/GOD-Chain/internal/godaddress"
	"github.com/ETKINGDOM/GOD-Chain/x/godbridge"
)

const MaxCustodyRequestBytes = 16 << 10

// Blank private request slots, not a signing packet or valid operation. Amounts
// and sequences use decimal strings, never imprecise JSON numbers. No address,
// timestamp, ID or signature is invented to make a template appear ready.
func CustodyRequestTemplate(action CustodyAction) ([]byte, error) {
	d := map[string]any{"version": 1, "action": action}
	switch action {
	case CustodyDeposit:
		d["nativeRecipient"], d["amountSmallestUnits"] = "", ""
	case CustodyPay, CustodyCancel:
		w := map[string]any{"id": "", "sequence": "", "nativeSender": "", "sourceRecipient": "",
			"amountSmallestUnits": "", "queuedAt": "", "status": string(godbridge.Queued)}
		if action == CustodyPay {
			w["status"], w["authorizedAt"] = string(godbridge.Authorized), ""
		}
		d["withdrawal"], d["approvals"] = w, []string{}
	case CustodyPause:
		d["controlNonce"], d["intake"], d["outflow"], d["approvals"] = "", false, false, []string{}
	default:
		return nil, ErrCustodyCall
	}
	raw, err := json.MarshalIndent(d, "", "  ")
	if err != nil {
		return nil, ErrCustodyCall
	}
	return append(raw, '\n'), nil
}

// Exact object keys at every protocol level. Escaped duplicates, case aliases,
// unknown fields, nulls and trailing JSON are refused before typed decoding.
func custodyObject(raw []byte, required, optional []string) (map[string]json.RawMessage, error) {
	d := json.NewDecoder(bytes.NewReader(raw))
	first, err := d.Token()
	if err != nil || first != json.Delim('{') {
		return nil, ErrCustodyCall
	}
	allowed := make(map[string]bool, len(required)+len(optional))
	for _, name := range append(append([]string(nil), required...), optional...) {
		allowed[name] = true
	}
	fields := make(map[string]json.RawMessage, len(allowed))
	for d.More() {
		key, err := d.Token()
		name, ok := key.(string)
		if err != nil || !ok || !allowed[name] || fields[name] != nil {
			return nil, ErrCustodyCall
		}
		var value json.RawMessage
		if d.Decode(&value) != nil || bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			return nil, ErrCustodyCall
		}
		fields[name] = value
	}
	last, err := d.Token()
	if err != nil || last != json.Delim('}') {
		return nil, ErrCustodyCall
	}
	for _, name := range required {
		if fields[name] == nil {
			return nil, ErrCustodyCall
		}
	}
	if _, err := d.Token(); err != io.EOF {
		return nil, ErrCustodyCall
	}
	return fields, nil
}

func custodyText(raw json.RawMessage) (string, error) {
	var text string
	if json.Unmarshal(raw, &text) != nil || text == "" || strings.TrimSpace(text) != text {
		return "", ErrCustodyCall
	}
	return text, nil
}

func custodyPositiveDecimal(raw json.RawMessage) (string, error) {
	text, err := custodyText(raw)
	if err != nil || len(text) > 78 || text[0] == '0' {
		return "", ErrCustodyCall
	}
	for _, digit := range text {
		if digit < '0' || digit > '9' {
			return "", ErrCustodyCall
		}
	}
	return text, nil
}

func custodyUint64(raw json.RawMessage) (uint64, error) {
	text, err := custodyPositiveDecimal(raw)
	if err != nil {
		return 0, ErrCustodyCall
	}
	n, err := strconv.ParseUint(text, 10, 64)
	if err != nil {
		return 0, ErrCustodyCall
	}
	return n, nil
}

func custodyAmount(raw json.RawMessage) (sdkmath.Int, error) {
	text, err := custodyPositiveDecimal(raw)
	if err != nil {
		return sdkmath.Int{}, ErrCustodyCall
	}
	n, ok := sdkmath.NewIntFromString(text)
	if !ok || !n.IsPositive() || n.GT(godbridge.TransferLimit()) {
		return sdkmath.Int{}, ErrCustodyCall
	}
	return n, nil
}

func custodyHex(raw json.RawMessage, size int) ([]byte, error) {
	text, err := custodyText(raw)
	if err != nil || len(text) != 2+2*size || !strings.HasPrefix(text, "0x") || strings.ToLower(text) != text {
		return nil, ErrCustodyCall
	}
	value, err := hex.DecodeString(text[2:])
	if err != nil {
		return nil, ErrCustodyCall
	}
	return value, nil
}

func custodyTime(raw json.RawMessage) (time.Time, error) {
	text, err := custodyText(raw)
	if err != nil || len(text) > 30 {
		return time.Time{}, ErrCustodyCall
	}
	t, err := time.Parse(time.RFC3339Nano, text)
	if err != nil || t.Unix() <= 0 || t.UTC().Format(time.RFC3339Nano) != text {
		return time.Time{}, ErrCustodyCall
	}
	return t.UTC(), nil
}

func custodyNative(raw json.RawMessage) ([20]byte, error) {
	var result [20]byte
	text, err := custodyText(raw)
	if err != nil {
		return result, ErrCustodyCall
	}
	value, err := godaddress.FromNative(text)
	if err != nil {
		return result, ErrCustodyCall
	}
	canonical, err := godaddress.ToNative(value)
	if err != nil || canonical != text {
		return result, ErrCustodyCall
	}
	copy(result[:], value)
	if result == [20]byte{} {
		return result, ErrCustodyCall
	}
	return result, nil
}

func custodyWithdrawal(raw json.RawMessage) (godbridge.Withdrawal, error) {
	w := godbridge.Withdrawal{}
	fields, err := custodyObject(raw, []string{"id", "sequence", "nativeSender", "sourceRecipient", "amountSmallestUnits", "queuedAt", "status"}, []string{"authorizedAt"})
	if err != nil {
		return w, ErrCustodyCall
	}
	id, err := custodyHex(fields["id"], 32)
	if err != nil {
		return w, ErrCustodyCall
	}
	copy(w.ID[:], id)
	w.Sequence, err = custodyUint64(fields["sequence"])
	if err != nil {
		return w, ErrCustodyCall
	}
	w.Sender, err = custodyNative(fields["nativeSender"])
	if err != nil {
		return w, ErrCustodyCall
	}
	recipient, err := custodyText(fields["sourceRecipient"])
	if err != nil {
		return w, ErrCustodyCall
	}
	w.Recipient, err = address(recipient)
	if err != nil {
		return w, ErrCustodyCall
	}
	w.Amount, err = custodyAmount(fields["amountSmallestUnits"])
	if err != nil {
		return w, ErrCustodyCall
	}
	w.QueuedAt, err = custodyTime(fields["queuedAt"])
	if err != nil {
		return w, ErrCustodyCall
	}
	status, err := custodyText(fields["status"])
	if err != nil {
		return w, ErrCustodyCall
	}
	w.Status = godbridge.Status(status)
	if w.Status == godbridge.Queued && fields["authorizedAt"] == nil {
		return w, nil
	}
	if w.Status != godbridge.Authorized || fields["authorizedAt"] == nil {
		return w, ErrCustodyCall
	}
	w.AuthorizedAt, err = custodyTime(fields["authorizedAt"])
	return w, err
}

// Parses one bounded detached private request; this does not verify its ID,
// configured quorum or any chain state. PrepareCustodyCall is the separate
// immutable binding/signature gate. Neither operation grants signing authority.
func ParseCustodyRequest(raw []byte) (request CustodyCallRequest, err error) {
	defer func() {
		if recover() != nil {
			request, err = CustodyCallRequest{}, ErrCustodyCall
		}
	}()
	if len(raw) == 0 || len(raw) > MaxCustodyRequestBytes || !utf8.Valid(raw) {
		return CustodyCallRequest{}, ErrCustodyCall
	}
	fields, err := custodyObject(raw, []string{"version", "action"}, []string{"nativeRecipient", "amountSmallestUnits", "withdrawal", "approvals", "controlNonce", "intake", "outflow"})
	if err != nil {
		return CustodyCallRequest{}, ErrCustodyCall
	}
	var version uint32
	if json.Unmarshal(fields["version"], &version) != nil || version != 1 || json.Unmarshal(fields["action"], &request.Action) != nil {
		return CustodyCallRequest{}, ErrCustodyCall
	}
	required := []string{"version", "action"}
	switch request.Action {
	case CustodyDeposit:
		required = append(required, "nativeRecipient", "amountSmallestUnits")
	case CustodyPay, CustodyCancel:
		required = append(required, "withdrawal", "approvals")
	case CustodyPause:
		required = append(required, "controlNonce", "intake", "outflow", "approvals")
	default:
		return CustodyCallRequest{}, ErrCustodyCall
	}
	if len(fields) != len(required) {
		return CustodyCallRequest{}, ErrCustodyCall
	}
	for _, name := range required {
		if fields[name] == nil {
			return CustodyCallRequest{}, ErrCustodyCall
		}
	}
	switch request.Action {
	case CustodyDeposit:
		if _, err = custodyNative(fields["nativeRecipient"]); err == nil {
			request.NativeRecipient, err = custodyText(fields["nativeRecipient"])
		}
		if err == nil {
			request.Amount, err = custodyAmount(fields["amountSmallestUnits"])
		}
	case CustodyPay, CustodyCancel:
		request.Withdrawal, err = custodyWithdrawal(fields["withdrawal"])
		if err == nil && (!unfinishedWithdrawal(request.Withdrawal) || request.Action == CustodyPay && request.Withdrawal.Status != godbridge.Authorized) {
			err = ErrCustodyCall
		}
	case CustodyPause:
		request.ControlNonce, err = custodyUint64(fields["controlNonce"])
		if err == nil && (json.Unmarshal(fields["intake"], &request.Intake) != nil || json.Unmarshal(fields["outflow"], &request.Outflow) != nil || !request.Intake && !request.Outflow) {
			err = ErrCustodyCall
		}
	}
	if err == nil && request.Action != CustodyDeposit {
		var signatures []json.RawMessage
		if json.Unmarshal(fields["approvals"], &signatures) != nil || len(signatures) < godbridge.Threshold || len(signatures) > godbridge.SignerCount {
			err = ErrCustodyCall
		} else {
			for _, raw := range signatures {
				var signature []byte
				signature, err = custodyHex(raw, 65)
				if err != nil {
					break
				}
				request.Approvals = append(request.Approvals, signature)
			}
		}
	}
	if err != nil {
		return CustodyCallRequest{}, ErrCustodyCall
	}
	return request, nil
}
