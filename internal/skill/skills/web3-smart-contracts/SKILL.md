# Web3 & Smart Contracts Development Skill

Comprehensive guide for Solidity smart contract development, security analysis, EVM internals, and the Rust Ethereum toolchain. Covers the full lifecycle from writing contracts through auditing and deployment.

---

## Overview

Web3 & Smart Contracts covers the full smart contract development lifecycle: writing, compiling, testing, deploying, verifying, and auditing Solidity contracts. Includes Foundry patterns, OpenZeppelin security, Slither analysis, EVM internals (revm), and the Rust Ethereum stack (alloy).

**When to use**: Building, debugging, or optimizing systems in this domain.

## 1. Smart Contract Development Lifecycle

```
Write (Solidity) → Compile (solc) → Test (forge test) → Audit (slither) → Deploy (forge script) → Verify (forge verify)
```

### 1.1 Project Setup with Foundry

```bash
forge init my-project && cd my-project
forge install OpenZeppelin/openzeppelin-contracts
forge install foundry-rs/forge-std
```

```toml
# foundry.toml
[profile.default]
src = "src"
out = "out"
libs = ["lib"]
solc = "0.8.28"
optimizer = true
optimizer_runs = 200
evm_version = "cancun"
fuzz = { runs = 256 }
invariant = { runs = 256, depth = 15 }
```

### 1.2 Contract Structure Best Practices

```solidity
// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import {Ownable} from "@openzeppelin/contracts/access/Ownable.sol";
import {ReentrancyGuard} from "@openzeppelin/contracts/utils/ReentrancyGuard.sol";
import {Pausable} from "@openzeppelin/contracts/utils/Pausable.sol";

// Order: types → state vars → events → errors → modifiers → constructor → external → public → internal → private
contract MyContract is Ownable, ReentrancyGuard, Pausable {
    // Pack storage: address (20) + uint64 (8) = 1 slot (32 bytes)
    address public token;
    uint64 public lastUpdate;

    // Custom errors (cheaper than require strings)
    error InsufficientBalance(uint256 available, uint256 required);
    error InvalidAddress();

    event Deposited(address indexed user, uint256 amount);

    modifier validAddress(address addr) {
        if (addr == address(0)) revert InvalidAddress();
        _;
    }

    constructor(address _token) Ownable(msg.sender) validAddress(_token) {
        token = _token;
    }

    function deposit(uint256 amount) external nonReentrant whenNotPaused {
        // Checks-Effects-Interactions pattern
        if (amount == 0) revert InsufficientBalance(0, amount);
        lastUpdate = uint64(block.timestamp);  // Effects before interactions
        emit Deposited(msg.sender, amount);
        IERC20(token).transferFrom(msg.sender, address(this), amount);
    }
}
```

---

## 2. Foundry Development Patterns

### 2.1 Testing Architecture

Foundry's test framework is built on revm and provides cheatcodes via a special address (`0x7109709ECfa91a80626fF3989D68f67F5b1DD12D`).

```solidity
import {Test} from "forge-std/Test.sol";

contract MyContractTest is Test {
    MyContract public target;
    address public user = makeAddr("user");

    function setUp() public {
        target = new MyContract(address(0x1234));
        vm.deal(user, 100 ether);
    }

    // Unit test
    function test_Deposit() public {
        vm.startPrank(user);
        target.deposit(100);
        vm.stopPrank();
        assertEq(target.balanceOf(user), 100);
    }

    // Fuzz test (Foundry generates random inputs)
    function testFuzz_Deposit(uint256 amount) public {
        vm.assume(amount > 0 && amount < type(uint128).max);
        vm.startPrank(user);
        target.deposit(amount);
        vm.stopPrank();
        assertEq(target.balanceOf(user), amount);
    }

    // Invariant test (property-based)
    function invariant_totalSupplyEqualsSumOfBalances() public {
        assertEq(target.totalSupply(), target.sumOfAllBalances());
    }

    // Fork test
    function testFork_Mainnet() public {
        vm.createSelectFork("mainnet", 18_000_000);
        address usdc = 0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48;
        assertGt(IERC20(usdc).totalSupply(), 0);
    }

    // Expect reverts/events
    function test_RevertOnZero() public {
        vm.expectRevert(MyContract.InsufficientBalance.selector);
        target.deposit(0);
    }
}
```

### 2.2 Key Cheatcodes Reference

| Category | Cheatcode | Purpose |
|----------|-----------|---------|
| **Calls** | `vm.prank(addr)` | Set msg.sender for next call |
| | `vm.startPrank(addr)` | Set msg.sender until stopPrank |
| | `vm.deal(addr, amount)` | Set ETH balance |
| **Storage** | `vm.load(addr, slot)` / `vm.store(addr, slot, val)` | Read/write raw storage |
| **State** | `vm.snapshot()` / `vm.revertTo(id)` | Snapshot/revert state |
| | `vm.roll(block)` / `vm.warp(timestamp)` | Set block number/timestamp |
| **Forking** | `vm.createFork(url)` / `vm.selectFork(id)` | Create/select forks |
| **Assertions** | `vm.expectRevert()` / `vm.expectEmit()` / `vm.expectCall()` | Expect outcomes |
| **Broadcast** | `vm.broadcast()` / `vm.startBroadcast()` | Record for deployment |

### 2.3 Deployment & Debugging

```bash
# Deploy
forge script script/Deploy.s.sol --rpc-url $SEPOLIA_RPC --broadcast --verify

# Debug
forge test -vvvv                                    # Full stack traces
forge test --match-test test_Deposit -vvvv          # Specific test debug
forge debug --debug test_Deposit                    # TUI debugger
forge snapshot --diff                               # Gas comparison
forge coverage --report lcov                        # Coverage for CI
```

---

## 3. OpenZeppelin Security Patterns

### 3.1 Token Standards

#### ERC-20 (Fungible)

```solidity
import {ERC20} from "@openzeppelin/contracts/token/ERC20/ERC20.sol";
import {ERC20Burnable} from "@openzeppelin/contracts/token/ERC20/extensions/ERC20Burnable.sol";
import {ERC20Permit} from "@openzeppelin/contracts/token/ERC20/extensions/ERC20Permit.sol";
import {ERC20Votes} from "@openzeppelin/contracts/token/ERC20/extensions/ERC20Votes.sol";

contract GovernanceToken is ERC20, ERC20Burnable, ERC20Permit, ERC20Votes {
    constructor() ERC20("Gov", "GOV") ERC20Permit("Gov") {
        _mint(msg.sender, 1_000_000e18);
    }

    function _update(address from, address to, uint256 value)
        internal override(ERC20, ERC20Votes) { super._update(from, to, value); }

    function nonces(address owner)
        public view override(ERC20Permit, Nonces) returns (uint256) {
        return super.nonces(owner);
    }
}
```

**Key patterns:** `_update()` is the unified hook for all state changes; `ERC20Permit` enables gasless approvals (EIP-2612); `ERC20Votes` adds delegation/checkpoints for governance; custom errors (ERC-6093) instead of revert strings.

#### ERC-721 (NFT)

```solidity
import {ERC721} from "@openzeppelin/contracts/token/ERC721/ERC721.sol";
import {ERC721Enumerable} from "@openzeppelin/contracts/token/ERC721/extensions/ERC721Enumerable.sol";
import {ERC721URIStorage} from "@openzeppelin/contracts/token/ERC721/extensions/ERC721URIStorage.sol";

contract MyNFT is ERC721, ERC721Enumerable, ERC721URIStorage {
    uint256 private _nextTokenId;
    constructor() ERC721("MyNFT", "MNFT") {}

    function safeMint(address to, string memory uri) public {
        uint256 tokenId = _nextTokenId++;
        _safeMint(to, tokenId);
        _setTokenURI(tokenId, uri);
    }

    function _update(address to, uint256 tokenId, address auth)
        internal override(ERC721, ERC721Enumerable) returns (address) {
        return super._update(to, tokenId, auth);
    }

    function _increaseBalance(address account, uint128 value)
        internal override(ERC721, ERC721Enumerable) {
        super._increaseBalance(account, value);
    }
}
```

**Key patterns:** `ERC721Consecutive` for gas-efficient batch minting; `ERC721Votes` for NFT governance; `checkOnERC721Received()` for safe transfers to contracts.

### 3.2 Access Control

```solidity
// Simple: Ownable
import {Ownable} from "@openzeppelin/contracts/access/Ownable.sol";

// Role-based: AccessControl (recommended for complex systems)
import {AccessControl} from "@openzeppelin/contracts/access/AccessControl.sol";
contract RoleBasedAuth is AccessControl {
    bytes32 public constant MINTER_ROLE = keccak256("MINTER_ROLE");
    bytes32 public constant PAUSER_ROLE = keccak256("PAUSER_ROLE");
    constructor() {
        _grantRole(DEFAULT_ADMIN_ROLE, msg.sender);
        _grantRole(MINTER_ROLE, msg.sender);
    }
    function mint(address to, uint256 amount) external onlyRole(MINTER_ROLE) {
        _mint(to, amount);
    }
}

// Secure admin: AccessControlDefaultAdminRules (2-step transfer with delay)
import {AccessControlDefaultAdminRules} from "@openzeppelin/contracts/access/extensions/AccessControlDefaultAdminRules.sol";
```

### 3.3 Proxy Patterns (Upgradeability)

```solidity
// UUPS (recommended for most cases)
import {UUPSUpgradeable} from "@openzeppelin/contracts/proxy/utils/UUPSUpgradeable.sol";
import {Initializable} from "@openzeppelin/contracts/proxy/utils/Initializable.sol";
import {OwnableUpgradeable} from "@openzeppelin/contracts/access/OwnableUpgradeable.sol";

contract MyContractV1 is Initializable, UUPSUpgradeable, OwnableUpgradeable {
    uint256 public value;

    /// @custom:oz-upgrades-unsafe-allow constructor
    constructor() { _disableInitializers(); }

    function initialize(address owner) public initializer {
        __Ownable_init(owner);
        __UUPSUpgradeable_init();
    }

    function _authorizeUpgrade(address newImplementation) internal override onlyOwner {}
}
```

**Upgrade Safety Rules:**
1. Never change storage layout order; always append new variables at the end
2. Use `__gap` arrays to reserve storage slots for future extensions
3. Use `@openzeppelin/upgrades-core` for storage layout validation
4. Test upgrades with `forge test --match-test Upgrade`

**Proxy Types:** Transparent (admin separation via ProxyAdmin), UUPS (implementation handles upgrade logic), Beacon (many proxies, one implementation — factory patterns).

### 3.4 Security Utilities

```solidity
import {ReentrancyGuard} from "@openzeppelin/contracts/utils/ReentrancyGuard.sol";              // Storage-based (deprecated in v6)
import {ReentrancyGuardTransient} from "@openzeppelin/contracts/utils/ReentrancyGuardTransient.sol"; // Transient storage (Cancun+)
import {Pausable} from "@openzeppelin/contracts/utils/Pausable.sol";                              // Emergency stop
import {SafeERC20} from "@openzeppelin/contracts/token/ERC20/utils/SafeERC20.sol";               // Handles non-standard tokens
import {ECDSA} from "@openzeppelin/contracts/utils/cryptography/ECDSA.sol";                       // Signature verification
import {EIP712} from "@openzeppelin/contracts/utils/cryptography/EIP712.sol";                     // Typed data signing
import {MerkleProof} from "@openzeppelin/contracts/utils/cryptography/MerkleProof.sol";           // Merkle proofs
import {SignatureChecker} from "@openzeppelin/contracts/utils/cryptography/SignatureChecker.sol"; // ECDSA + ERC-1271
import {Multicall} from "@openzeppelin/contracts/utils/Multicall.sol";                            // Batched calls
import {Create2} from "@openzeppelin/contracts/utils/Create2.sol";                                // Deterministic deployment
```

---

## 4. Slither Static Analysis

### 4.1 Architecture

Slither builds a SlithIR intermediate representation from Solidity AST for data-flow analysis.

```
Solidity Source → solc AST → Slither Core (Contract/Function/Variable)
    → SlithIR (SSA form) → Detectors → Output
```

**Core Classes:** `SlitherCore` (entry point), `SlitherCompilationUnit`, `Contract`, `FunctionContract`, `Node` (CFG), SlithIR (intermediate representation).

### 4.2 Detector Categories

| Category | Key Detectors | Severity |
|----------|--------------|----------|
| **Reentrancy** | `reentrancy-eth`, `reentrancy-no-eth`, `reentrancy-events` | HIGH |
| **Access Control** | `missing-access-control`, `arbitrary-send-eth`, `suicidal` | HIGH |
| **Unchecked Calls** | `unchecked-transfer`, `unchecked-send`, `low-level-calls` | MEDIUM |
| **State Variables** | `uninitialized-state`, `uninitialized-storage`, `could-be-immutable` | MEDIUM |
| **Statements** | `tx-origin`, `timestamp-dependence`, `divide-before-multiply`, `msg-value-loop` | MEDIUM |
| **Compiler Bugs** | `solc-version`, `assembly`, `incorrect-equality` | LOW-MEDIUM |
| **Optimization** | `constable-states`, `external-function`, `redundant-statements` | OPTIMIZATION |

### 4.3 Usage

```bash
slither .                                                    # Full analysis
slither . --detect high                                      # High severity only
slither . --json results.json                                # JSON for CI
slither . --exclude naming-convention,assembly               # Exclude categories
slither-check-upgradeability . V1 --new-contract-name V2     # Proxy checks
slither-read-storage . --contract-name MyContract --rpc-url $RPC  # Storage layout
```

### 4.4 Writing Custom Detectors

```python
from slither.detectors.abstract_detector import AbstractDetector, DetectorClassification

class MyDetector(AbstractDetector):
    ARGUMENT = "my-detector"
    HELP = "Description"
    IMPACT = DetectorClassification.HIGH
    CONFIDENCE = DetectorClassification.MEDIUM
    WIKI_TITLE = "My Detector"
    WIKI_DESCRIPTION = "Detailed description"
    WIKI_EXPLOIT_SCENARIO = "Attack scenario"
    WIKI_RECOMMENDATION = "Fix guidance"

    def _detect(self):
        results = []
        for contract in self.contracts:
            for function in contract.functions_declared:
                for node in function.nodes:
                    for ir in node.irs:  # Use node.irs for most analyses
                        pass
        return results
```

### 4.5 SlithIR Tips

- **`node.irs`** for most detectors (call analysis, state reads/writes)
- **`node.irs_ssa`** only for precise data-flow (taint analysis, reassignment chains)
- SSA variables: `.index` (x_0, x_1), `.non_ssa_version` for original
- `Phi` operations merge SSA versions at control flow joins
- Data dependency: `is_dependent(var, source, context)` from `slither.analyses.data_dependency`
- Per-definition: `contracts_derived + functions_declared`; reachability: `contracts_derived + functions`

---

## 5. EVM Internals (revm)

### 5.1 Architecture

revm is the Rust EVM used by Foundry, Hardhat, Reth, and Optimism.

```
crates/
├── primitives/         # U256, Address, B256, hardfork specs
├── bytecode/           # Bytecode analysis, EOF validation
├── interpreter/        # Opcode execution, stack, memory, gas
├── context/            # Default context, journal, EVM container
├── context-interface/  # Traits for context, environment, journal
├── handler/            # Mainnet execution flow, frames, validation
├── database/           # Database implementations
├── state/              # Account/storage/state types
├── precompile/         # Precompiled contracts
├── inspector/          # Tracing and debugging APIs
```

### 5.2 Gas Costs

| Operation | Gas | Notes |
|-----------|-----|-------|
| `SLOAD` (cold) | 2100 | First access in transaction |
| `SLOAD` (warm) | 100 | Subsequent access |
| `SSTORE` (non-zero → non-zero) | 2900 | |
| `SSTORE` (zero → non-zero) | 22100 | Storage creation |
| `SSTORE` (non-zero → zero) | 2900 + 4800 refund | |
| `CALL` (cold) | 2600 | First call to address |
| `CREATE` | 32000 | |
| Memory expansion | 3*words + words²/512 | Quadratic |
| Transaction base | 21000 | |

### 5.3 Precompiled Contracts

| Address | Function | Gas |
|---------|----------|-----|
| `0x01` | ecrecover | 3000 flat |
| `0x02` | SHA-256 | 60 + 12*words |
| `0x03` | RIPEMD-160 | 600 + 120*words |
| `0x05` | modexp | Complex formula |
| `0x06` | ecAdd (BN254) | 150 |
| `0x07` | ecMul (BN254) | 6000 |
| `0x08` | ecPairing (BN254) | 45000 + 34000*points |
| `0x0a` | KZG point eval | 50000 |

### 5.4 EVM Security Implications

```solidity
// Storage collision in proxies — use ERC-1967 deterministic slots
bytes32 private constant IMPLEMENTATION_SLOT =
    bytes32(uint256(keccak256("eip1967.proxy.implementation")) - 1);

// Transient storage (EIP-1153, Cancun) — perfect for reentrancy locks
modifier nonReentrant() {
    assembly {
        if tload(0) { revert(0, 0) }
        tstore(0, 1)
        _
        tstore(0, 0)
    }
}

// Cold vs warm: first access costs 2100 gas, subsequent 100 gas per transaction
// Memory expansion: quadratic cost makes large memory expensive
// Call stipend (2300 gas) limits what callee can do in ETH transfers
```

---

## 6. Rust Ethereum Stack (alloy)

### 6.1 Architecture

alloy is the modern Rust Ethereum library replacing ethers-rs, used by Foundry and Reth.

```
crates/
├── primitives/        # Address, B256, U256
├── sol-types/         # Solidity ABI encoding/decoding
├── transport/         # Transport trait (tower::Service based)
├── transport-http/    # HTTP (reqwest/hyper)
├── transport-ws/      # WebSocket
├── transport-ipc/     # IPC
├── rpc-client/        # High-level RPC client
├── rpc-types/         # Ethereum RPC type definitions
├── consensus/         # Transaction types, receipts, headers
├── signer/            # Transaction signing trait
├── signer-local/      # Local key signing
├── provider/          # Provider trait + implementations
├── contract/          # Contract interaction via ABI
├── network/           # Network abstraction (Ethereum, Optimism)
```

### 6.2 Key Abstractions

```rust
// Transport: tower::Service based, composable middleware
pub trait Transport: Service<RequestPacket, Response = ResponsePacket,
    Error = TransportError, Future = TransportFut<'static>> + Send + Sync + 'static {}

// Provider: high-level RPC interface
use alloy::providers::{Provider, ProviderBuilder};
let provider = ProviderBuilder::new()
    .with_recommended_fillers()
    .on_http("http://localhost:8545".parse()?);
let balance = provider.get_balance(address).await?;

// Contract interaction
alloy::sol! {
    #[sol(rpc)]
    interface IERC20 {
        function transfer(address to, uint256 amount) returns (bool);
        function balanceOf(address account) view returns (uint256);
    }
}
let token = IERC20::new(token_address, &provider);
let balance = token.balanceOf(user).call().await?;

// Signing
use alloy::signers::local::PrivateKeySigner;
let signer = PrivateKeySigner::from_bytes(&key_bytes)?;
let provider = ProviderBuilder::new()
    .wallet(EthereumWallet::from(signer))
    .on_http(rpc_url);
```

---

## 7. DeFi Patterns

### 7.1 AMM (Constant Product)

```solidity
function swap(uint256 amountIn, address tokenIn) external returns (uint256 amountOut) {
    (uint256 reserve0, uint256 reserve1) = getReserves();
    (uint256 reserveIn, uint256 reserveOut) = tokenIn == token0
        ? (reserve0, reserve1) : (reserve1, reserve0);

    uint256 amountInWithFee = amountIn * 997;  // 0.3% fee
    amountOut = (amountInWithFee * reserveOut) / (reserveIn * 1000 + amountInWithFee);

    IERC20(tokenIn).transferFrom(msg.sender, address(this), amountIn);
    IERC20(tokenOut).transfer(msg.sender, amountOut);
    _update(reserve0 + delta0, reserve1 + delta1);
}
```

### 7.2 Oracle Pattern

```solidity
function getPrice(AggregatorV3Interface feed) external view returns (int256) {
    (, int256 price,, uint256 updatedAt, uint80 answeredInRound) = feed.latestRoundData();
    require(updatedAt > block.timestamp - 1 hours, "stale price");
    require(answeredInRound >= roundId, "incomplete round");
    require(price > 0, "invalid price");
    return price;
}
```

### 7.3 Flash Loan Pattern

```solidity
function flashLoan(address receiver, uint256 amount) external {
    uint256 before = token.balanceOf(address(this));
    token.transfer(receiver, amount);
    IFlashLoanReceiver(receiver).executeOperation(amount, fee, data);
    require(token.balanceOf(address(this)) >= before + fee, "not repaid");
}
```

---

## 8. Security Audit Checklist

### 8.1 Pre-Audit Analysis

```bash
slither . --json audit-results.json
slither . --detect reentrancy-eth,unchecked-transfer,arbitrary-send-eth,uninitialized-state,tx-origin
slither-check-upgradeability . V1 --new-contract-name V2
slither-read-storage . --contract-name MyContract
```

### 8.2 Critical Vulnerability Checklist

| # | Category | Check | Tool |
|---|----------|-------|------|
| 1 | **Reentrancy** | All external calls protected by nonReentrant or CEI | Slither |
| 2 | **Access Control** | Sensitive functions have modifiers | Slither |
| 3 | **Unchecked Returns** | Low-level calls check return values | Slither |
| 4 | **Oracle Manipulation** | Price feeds have staleness + sanity checks | Manual |
| 5 | **Flash Loan Attacks** | No spot price reliance within single tx | Manual |
| 6 | **Storage Collision** | Proxy layout doesn't overlap | Slither |
| 7 | **Delegatecall Safety** | Targets are trusted and immutable | Manual |
| 8 | **DoS Vectors** | No unbounded user-controlled loops | Manual |
| 9 | **Signature Replay** | Nonces used, domain separator includes chain ID | Manual |
| 10 | **Precision Loss** | Division before multiplication avoided | Slither |
| 11 | **tx.origin** | Never used for authorization | Slither |
| 12 | **Block Timestamp** | Not used for critical logic (±15s) | Slither |
| 13 | **Front-running** | Commit-reveal or private mempool for sensitive ops | Manual |
| 14 | **Griefing** | No forced ether via selfdestruct | Manual |
| 15 | **Initialization** | Initializers protected against re-init | Slither |

### 8.3 Testing Strategy

```
Unit Tests (100% coverage)
    ├── Happy path + edge cases (0, max, boundaries)
    └── Access control verification

Fuzz Tests (property-based)
    ├── Arithmetic invariants
    └── State machine invariants

Invariant Tests (protocol-wide)
    ├── Solvency: deposits >= withdrawals
    └── Conservation: balances sum correctly

Fork Tests (mainnet state)
    ├── Integration with deployed protocols
    └── Gas under real conditions
```

---

## 9. Advanced Patterns

### 9.1 CREATE2 (Deterministic Deployment)

```solidity
import {Create2} from "@openzeppelin/contracts/utils/Create2.sol";
address addr = Create2.deploy(salt, type(MyContract).creationCode);
// Same address across all chains with same salt
```

### 9.2 EIP-712 Typed Data Signing

```solidity
import {EIP712} from "@openzeppelin/contracts/utils/cryptography/EIP712.sol";
import {ECDSA} from "@openzeppelin/contracts/utils/cryptography/ECDSA.sol";

bytes32 private constant PERMIT_TYPEHASH = keccak256(
    "Permit(address owner,address spender,uint256 value,uint256 nonce,uint256 deadline)"
);

bytes32 structHash = keccak256(abi.encode(PERMIT_TYPEHASH, owner, spender, value, nonce, deadline));
bytes32 digest = _hashTypedDataV4(structHash);
address signer = ECDSA.recover(digest, v, r, s);
```

### 9.3 Diamond Pattern (EIP-2535)

For contracts exceeding 24KB, split logic across "facet" contracts behind a single proxy with `mapping(bytes4 => address)` dispatch.

---

## 10. Tool Integration Matrix

| Tool | Purpose | When to Use |
|------|---------|-------------|
| **forge** | Build, test, deploy | Every dev cycle |
| **cast** | RPC interaction, ABI encoding | Ad-hoc queries, scripts |
| **anvil** | Local dev node | Local testing, forking |
| **chisel** | Solidity REPL | Quick experiments |
| **slither** | Static analysis | Pre-commit, CI, audit |
| **alloy** | Rust Ethereum lib | Building Rust tooling |
| **revm** | EVM implementation | Custom EVM, tooling |
| **mythril** | Symbolic execution | Deep security analysis |
| **echidna** | Fuzzing | Property-based testing |
| **certora** | Formal verification | Critical invariant proofs |

## Quick Reference

```bash
# Foundry
forge build && forge test -vvvv
forge script script/Deploy.s.sol --broadcast --verify
cast call <addr> "balanceOf(address)(uint256)" <user>

# Slither
slither . --detect high
slither-check-upgradeability . V1

# Anvil
anvil --fork-url $MAINNET_RPC
```

## Anti-Patterns Found

1. **Using `require` strings instead of custom errors** — wastes gas; use `revert CustomError()` (OpenZeppelin ERC-6093)
2. **Not using ReentrancyGuard on external calls** — always protect state-changing functions that call external contracts
3. **Storing arrays in storage without length checks** — unbounded loops can hit gas limits
4. **Not accounting for cold storage costs** — EIP-2929 makes first access 2100 gas vs 100 for warm
5. **Using `tx.origin` for authentication** — phishing vulnerability; use `msg.sender`
6. **Not testing with fork testing** — always test against mainnet state for DeFi protocols
7. **Ignoring MEV in price calculations** — sandwich attacks on DEX interactions
8. **Not using SafeERC20 for token transfers** — some tokens don't return bool


---

## Decision tree

See **When to use** above.

## Workflow

1. Read the skill body above.
2. Identify the relevant section for your task.
3. Apply the patterns and examples provided.
4. Verify against the listed anti-patterns.

## Examples

Examples are interleaved throughout the skill body above.

## Anti-patterns

See the **When to use** criteria above. The skill is *not* applied when:
- The task is outside the skill's declared scope.
- Simpler alternatives exist.

## Failure modes

- **Misapplied scope**: invoking the skill for tasks it doesn't cover.
- **Outdated reference**: real-world library APIs may have shifted since synthesis.
- **Pattern drift**: the skill's patterns describe idealized APIs, not exact production code.

## Related skills

- `software-architecture-go`

