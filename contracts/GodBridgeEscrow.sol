// SPDX-License-Identifier: UNLICENSED
pragma solidity 0.8.37;

interface IGodAsset {
    function decimals() external view returns (uint8);
    function totalSupply() external view returns (uint256);
    function balanceOf(address) external view returns (uint256);
    function transfer(address, uint256) external returns (bool);
    function transferFrom(address, address, uint256) external returns (bool);
}

/// Isolated source-custody prototype, NOT a deployed or audited RH bridge.
/// No owner, upgrades, rescue transfer, resume, mint or signer rotation exists.
/// Five independent signers must verify native finality before approving pay;
/// source receipts must become final before native release or refund.
contract GodBridgeEscrow {
    uint256 public constant FIXED_SUPPLY = 1_000_000_000 * 1e18;
    uint256 public constant EXPOSURE_LIMIT = 10_000 * 1e18;
    uint256 public constant TRANSFER_LIMIT = 1_000 * 1e18;
    uint256 public constant OUTFLOW_LIMIT = 1_000 * 1e18;
    uint256 public constant DELAY = 1 days;
    uint256 public constant MAX_SPAN = 1024;
    // Public secp256k1 group constant, not signing material.
    uint256 private constant HALF_ORDER = 57896044618658097711785492504343953926418782139537452191302581570759080747168;

    IGodAsset public immutable token;
    bytes32 public immutable assetID;
    string public sourceChain;
    string public nativeChain;
    address[7] public signers;
    mapping(address => uint8) private signerIndex;
    bool private entered;
    bool public intakePaused;
    bool public outflowPaused;
    uint64 public controlNonce;
    uint64 public nextDeposit = 1;
    uint64 public head = 1;
    uint256 public escrowed;

    enum Outcome { None, Paid, Cancelled }
    struct Terminal { bytes32 id; Outcome outcome; }
    mapping(uint64 => Terminal) public terminal;
    struct WindowEntry { uint64 when; uint256 amount; }
    WindowEntry[1024] private window;
    uint16 private windowStart;
    uint16 private windowCount;
    uint256 public reservedOutflow;

    struct Withdrawal {
        uint64 sequence;
        bytes20 sender;
        bytes20 recipient;
        uint256 amount;
        uint64 queuedSeconds;
        uint32 queuedNanos;
        bytes32 id;
    }

    event Deposited(uint64 indexed sequence, address indexed sender, bytes20 recipient, uint256 amount);
    event Paid(uint64 indexed sequence, bytes32 indexed id, bytes20 recipient, uint256 amount);
    event Cancelled(uint64 indexed sequence, bytes32 indexed id);
    event Paused(uint64 indexed nonce, bool intake, bool outflow);

    modifier locked() {
        require(!entered, "reentrant"); entered = true; _; entered = false;
    }

    constructor(address asset, uint256 expectedChainID, string memory source, string memory nativeName, address[7] memory members) {
        require(asset != address(0) && asset.code.length > 0 && expectedChainID > 0 && expectedChainID == block.chainid, "asset or chain");
        require(_chainName(source) && _chainName(nativeName) && keccak256(bytes(source)) != keccak256(bytes(nativeName)), "domain");
        token = IGodAsset(asset);
        require(token.decimals() == 18 && token.totalSupply() == FIXED_SUPPLY && token.balanceOf(address(this)) == 0, "asset accounting");
        sourceChain = source; nativeChain = nativeName;
        for (uint8 i; i < 7; ++i) {
            require(members[i] != address(0) && (i == 0 || uint160(members[i]) > uint160(members[i-1])), "signer set");
            signers[i] = members[i]; signerIndex[members[i]] = i+1;
        }
        assetID = sha256(abi.encodePacked(_text("GOD Chain bridge asset"), uint32(1), expectedChainID, asset, address(this), uint8(18)));
    }

    function _chainName(string memory name) private pure returns (bool) {
        bytes memory b = bytes(name);
        if (b.length == 0 || b.length > 50) return false;
        for (uint256 i; i < b.length; ++i) {
            uint8 c = uint8(b[i]);
            if (!(c >= 65 && c <= 90 || c >= 97 && c <= 122 || c >= 48 && c <= 57 || c == 46 || c == 95 || c == 45)) return false;
        }
        return true;
    }

    function _text(string memory s) private pure returns (bytes memory) {
        return abi.encodePacked(uint16(bytes(s).length), bytes(s));
    }

    function _domain(string memory action) private view returns (bytes memory) {
        return abi.encodePacked(_text("GOD Chain bridge attestation"), uint32(1), _text(sourceChain), _text(nativeChain), assetID, _text(action));
    }

    function _withdrawal(Withdrawal calldata w, string memory action) private view returns (bytes memory) {
        require(w.sequence != 0 && w.sender != bytes20(0) && w.recipient != bytes20(0) && w.amount > 0 && w.amount <= TRANSFER_LIMIT, "withdrawal");
        require(w.queuedSeconds > 0 && w.queuedSeconds <= 253402300799 && w.queuedNanos < 1_000_000_000, "time");
        return abi.encodePacked(_domain(action), w.sequence, w.sender, w.recipient, w.amount, w.queuedSeconds, w.queuedNanos);
    }

    function withdrawalID(Withdrawal calldata w) public view returns (bytes32) {
        return sha256(_withdrawal(w, "withdrawal-id"));
    }

    function authorizationDigest(Withdrawal calldata w) public view returns (bytes32) {
        require(withdrawalID(w) == w.id, "id");
        return sha256(bytes.concat(_withdrawal(w, "withdrawal-authorize"), w.id));
    }

    function cancellationRequestDigest(Withdrawal calldata w) public view returns (bytes32) {
        require(withdrawalID(w) == w.id, "id");
        return sha256(bytes.concat(_withdrawal(w, "withdrawal-cancel-request"), w.id));
    }

    function _verify(bytes32 digest, bytes[] calldata approvals) private view {
        require(approvals.length >= 5 && approvals.length <= 7, "quorum");
        uint256 seen;
        for (uint256 i; i < approvals.length; ++i) {
            bytes calldata signature = approvals[i];
            require(signature.length == 65, "signature length");
            bytes32 r; bytes32 s; uint8 v;
            assembly { r := calldataload(signature.offset) s := calldataload(add(signature.offset,32)) v := byte(0,calldataload(add(signature.offset,64))) }
            require(v < 2 && uint256(s) > 0 && uint256(s) <= HALF_ORDER, "canonical signature");
            address member = ecrecover(digest, v+27, r, s);
            uint8 index = signerIndex[member];
            require(index > 0 && seen & (uint256(1) << index) == 0, "distinct signer");
            seen |= uint256(1) << index;
        }
    }

    function _assetInvariant() private view {
        require(token.decimals() == 18 && token.totalSupply() == FIXED_SUPPLY && token.balanceOf(address(this)) >= escrowed, "backing accounting");
    }

    // Only the actual received amount is credited. Fee-on-transfer, rebasing
    // and arbitrary supply controls are unsupported and need token review.
    function deposit(uint256 amount, bytes20 recipient) external locked {
        require(!intakePaused && recipient != bytes20(0) && amount > 0 && amount <= TRANSFER_LIMIT, "deposit");
        require(nextDeposit < type(uint64).max && escrowed+amount <= EXPOSURE_LIMIT, "exposure");
        _assetInvariant();
        uint256 beforeBalance = token.balanceOf(address(this));
        _callToken(abi.encodeCall(IGodAsset.transferFrom, (msg.sender, address(this), amount)));
        require(token.balanceOf(address(this)) == beforeBalance+amount, "received amount");
        escrowed += amount;
        _assetInvariant();
        emit Deposited(nextDeposit++, msg.sender, recipient, amount);
    }

    function _trimAndReserve(uint256 amount) private {
        while (windowCount > 0 && uint256(window[windowStart].when)+DELAY <= block.timestamp) {
            reservedOutflow -= window[windowStart].amount;
            windowStart = uint16((uint256(windowStart)+1)%MAX_SPAN); --windowCount;
        }
        require(windowCount < MAX_SPAN && reservedOutflow+amount <= OUTFLOW_LIMIT && block.timestamp <= type(uint64).max, "rolling outflow");
        uint16 index = uint16((uint256(windowStart)+windowCount)%MAX_SPAN);
        window[index] = WindowEntry(uint64(block.timestamp),amount); ++windowCount; reservedOutflow += amount;
    }

    function pay(Withdrawal calldata w, bytes[] calldata approvals) external locked {
        require(!outflowPaused && w.sequence == head && terminal[w.sequence].outcome == Outcome.None, "queue or terminal");
        _verify(authorizationDigest(w), approvals);
        // RH timestamps have second precision. Round the source nanosecond
        // deadline up, never down, so payment cannot precede the public delay.
        require(block.timestamp >= uint256(w.queuedSeconds)+DELAY+(w.queuedNanos > 0 ? 1 : 0), "delay");
        address recipient = address(w.recipient);
        require(recipient != address(this) && recipient != address(token), "recipient");
        _assetInvariant(); require(escrowed >= w.amount, "escrow");
        _trimAndReserve(w.amount);
        terminal[w.sequence] = Terminal(w.id,Outcome.Paid); escrowed -= w.amount;
        uint256 beforeEscrow = token.balanceOf(address(this));
        uint256 beforeRecipient = token.balanceOf(recipient);
        _callToken(abi.encodeCall(IGodAsset.transfer, (recipient,w.amount)));
        require(token.balanceOf(address(this))+w.amount == beforeEscrow && token.balanceOf(recipient) == beforeRecipient+w.amount, "paid amount");
        _assetInvariant(); emit Paid(w.sequence,w.id,w.recipient,w.amount); _advance();
    }

    // Records an irreversible tombstone without paying or refunding tokens.
    // Native refund needs a separate finalized receipt attestation, NOT these
    // request signatures. Paid and cancelled share one sequence namespace.
    function cancel(Withdrawal calldata w, bytes[] calldata approvals) external locked {
        require(w.sequence >= head && uint256(w.sequence)-head < MAX_SPAN && terminal[w.sequence].outcome == Outcome.None, "cancel queue");
        _verify(cancellationRequestDigest(w),approvals);
        terminal[w.sequence] = Terminal(w.id,Outcome.Cancelled);
        emit Cancelled(w.sequence,w.id); _advance();
    }

    function _advance() private {
        for (uint256 i; i < MAX_SPAN && terminal[head].outcome != Outcome.None; ++i) {
            require(head < type(uint64).max, "sequence exhausted"); ++head;
        }
    }

    function pause(uint64 nonce, bool intake, bool outflow, bytes[] calldata approvals) external locked {
        require(controlNonce < type(uint64).max && nonce == controlNonce+1 && (intake || outflow), "pause nonce");
        require((!intakePaused || intake) && (!outflowPaused || outflow) && (intakePaused != intake || outflowPaused != outflow), "pause only");
        uint8 flags = (intake ? 1 : 0) | (outflow ? 2 : 0);
        _verify(sha256(abi.encodePacked(_domain("pause-only"),nonce,flags)),approvals);
        controlNonce = nonce; intakePaused = intake; outflowPaused = outflow;
        emit Paused(nonce,intake,outflow);
    }

    function _callToken(bytes memory callData) private {
        (bool success, bytes memory result) = address(token).call(callData);
        require(success && (result.length == 0 || result.length == 32 && abi.decode(result,(bool))), "token transfer");
    }
}
