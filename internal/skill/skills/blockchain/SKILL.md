# Skill: blockchain

> Smart-contract architecture, security audits, gas optimisation,
> upgrade patterns, oracle integration. Code is law — and bugs
> drain treasuries in one transaction.

## Decision tree

```
Project starts (smart contract + value)
        │
        ▼
[Step 1] Chain selection         ── EVM / Solana / Move / Cairo / Cosmos
        │
        ▼
[Step 2] Architecture            ── docs/blockchain/architecture.md
        │                              (contracts, upgrade, access control)
        ▼
[Step 3] Threat model            ── docs/blockchain/threat-model.md
        │                              (re-entrancy, oracle, governance)
        ▼
[Step 4] Implementation          ── Foundry / Hardhat / Anchor
        │                              (with ≥95% coverage)
        ▼
[Step 5] Internal review         ── static analysis + peer review
        │
        ▼
[Step 6] External audit          ── docs/blockchain/security-audit.md
        │                              (firm + findings + remediation)
        ▼
[Step 7] Testnet deployment      ── public testnet + bug bounty
        │
        ▼
[Step 8] Upgrade plan            ── docs/blockchain/upgrade-plan.md
        │                              (proxy / timelock / governance)
        ▼
[Step 9] Mainnet deployment      ── multisig + timelock + monitoring
        │
        ▼
[Step 10] Continuous monitoring  ── events, TVL, anomalies
```

## Workflow

### Step 1: Chain selection

| Chain family | Strengths | Weaknesses | Best for |
|--------------|-----------|------------|----------|
| **EVM (Ethereum L1)** | Largest ecosystem; tooling; liquidity | High gas (L1); MEV | High-value DeFi; L1 settlement |
| **EVM L2 (Arbitrum, Optimism, Base)** | Low gas; EVM compat | Bridge UX; sequencer risk | Consumer apps; medium-value DeFi |
| **EVM sidechain (Polygon, BSC)** | Low gas; high TPS | More centralised; validator set smaller | Gaming; microtransactions |
| **Solana** | High TPS; low fees; fast finality | Less mature tooling; network outages | Consumer; high-frequency; DePIN |
| **Move (Aptos, Sui)** | Resource-oriented model; safer | Newer; smaller ecosystem | Asset tokenisation; NFTs at scale |
| **Cairo (Starknet)** | Validity proofs (ZK); low L1 cost | Newer; tooling immature | ZK-rollup apps; privacy |
| **Cosmos SDK** | App-chain sovereignty | Bootstrapping validator set | Inter-chain protocols |
| **Substrate (Polkadot)** | Custom runtimes; parachains | Bootstrapping | Interoperability |

Decision factors:
- **Existing users / liquidity**: EVM has the most.
- **Throughput / cost**: Solana / L2 / sidechains win.
- **Security model**: Bitcoin / Ethereum L1 are the most decentralised.
- **Tooling**: EVM is the most mature.

### Step 2: Architecture

`docs/blockchain/architecture.md`:

- **Chain + L1/L2/sidechain choice + rationale**
- **Contract structure**: which contracts; what each owns;
  how they communicate
- **Access control**: who can do what (owner, admin, governance,
  emergency)
- **Value flow**: deposits, withdrawals, fees, rewards — drawn
  as a diagram
- **Upgrade strategy**: proxy pattern; timelock; multisig;
  emergency pause
- **Oracles**: which feeds; staleness thresholds; deviation checks
- **Integration points**: frontend; indexers; relayers; bridges

The architecture MUST be signed off BEFORE writing any contract.
"Build the wrong contract" is a multi-month, multi-audit loss.

### Step 3: Threat model

Smart-contract-specific threats:

| Category | Threat | Example |
|----------|--------|---------|
| **Re-entrancy** | External call calls back into the contract before state is updated | The DAO hack (2016) |
| **Oracle manipulation** | Price feed is spoofed via low-liquidity pool | bZx, Mango Markets |
| **Flash loan attack** | Borrow → manipulate → profit → repay in one tx | Cream, Beanstalk |
| **Governance attack** | Accumulate votes, pass malicious proposal | Compound Proposal 62 (rescued) |
| **Front-running / MEV** | Sandwich attacks; back-running | Every public mempool |
| **Integer overflow** | Math wraps around (pre-0.8 Solidity) | BeautyChain BEC |
| **Access control bug** | Function meant to be admin-only is public | Parity Wallet (frozen funds) |
| **Logic bug** | Edge case in business logic | Cream, Cover, bZx |
| **Logic bomb in proxy** | Implementation contract has a `selfdestruct` / `transfer` | Parity Wallet (again) |
| **Frontrunning of approvals** | Approve, victim gets phished to spend | Widely exploited |
| **Block stuffing** | Fill blocks with attacker txs to delay victim | Fomo3D-ish patterns |

For each: likelihood (low / medium / high), impact ($, trust),
mitigation (existing controls), gap.

### Step 4: Implementation

Tooling:

| Framework | Language | Strengths |
|-----------|----------|-----------|
| **Foundry** | Solidity | Fast; fuzz testing; cheatcodes; forge-std |
| **Hardhat** | Solidity | Plugin ecosystem; TypeScript integration |
| **Truffle** | Solidity | Mature; declining usage |
| **Anchor** | Rust (Solana) | IDL generation; type-safe; accounts model |
| **Aptos Move CLI** | Move | Compile + test + publish |
| **Sui Move CLI** | Move | Similar to Aptos |
| **Cairo (Scarb)** | Cairo | Native to Starknet |

**Patterns to use**:

- **Checks-Effects-Interactions** (CEI): validate, update state,
  THEN external call. Prevents re-entrancy.
- **Re-entrancy guard** (OpenZeppelin's `ReentrancyGuard`) for
  any function with external calls.
- **Pull over push**: let users withdraw rather than pushing to
  them. Avoids gas griefing and re-entrancy.
- **Use OpenZeppelin contracts** for ERC20, ERC721, access
  control, proxy, etc. Don't roll your own.
- **Solidity 0.8+**: checked arithmetic by default.
- **Use SafeERC20** for token transfers (handles non-standard
  returns).
- **Commit-reveal** for sensitive user input (bids, votes).
- **Merkle proofs** for airdrops / allowlists (gas-efficient).

**Anti-patterns to avoid**:

- `tx.origin` for auth (use `msg.sender`).
- `block.timestamp` for randomness (miners can manipulate within
  ~15s).
- Unbounded loops (DoS via gas).
- `selfdestruct` in any modern contract.
- `delegatecall` to user-supplied addresses.

### Step 5: Internal review

Before external audit:

| Tool | Catches |
|------|---------|
| **Slither** | Static analysis; common bugs; gas |
| **Mythril** | Symbolic execution; re-entrancy, overflow |
| **Echidna** | Property-based / fuzz testing |
| **Foundry invariant** | Invariant testing in Foundry |
| **Certora** | Formal verification of properties |
| **Manual review** | Business logic; access control |

Run all of the above. Document findings + remediation.

### Step 6: External audit

**Required** for any contract with value at risk >$100k.

| Audit firm | Speciality | Cost range |
|-----------|-----------|-----------|
| **Trail of Bits** | EVM, formal verification | $$$$ |
| **OpenZeppelin** | EVM, standards, security | $$$ |
| **Spearbit** | EVM, competitive audit | $$$ |
| **Code4rena** | EVM, competitive audit contests | $$ |
| **Cantina** | EVM, competitive + private | $$$ |
| **OtterSec** | Solana | $$$ |
| **Trail of Bits** | Multi-chain | $$$$ |
| **Runtime Verification** | Formal verification | $$$$ |

For value at risk:
- **< $100k**: internal review may suffice; bug bounty recommended.
- **$100k–$10M**: single external audit + bug bounty.
- **$10M–$1B**: multi-firm audit + formal verification on
  critical paths + substantial bug bounty ($1M+).
- **>$1B**: all of the above + ongoing audit retainer + dedicated
  security team.

Audit deliverables: scope, methodology, findings (severity:
critical/high/medium/low/informational), remediation status,
sign-off.

### Step 7: Testnet deployment

- Deploy to public testnet (Sepolia, Base Sepolia, devnet).
- Run a bug bounty program (Code4rena, Sherlock, Immunefi).
- Encourage responsible disclosure; have a process.
- Test all upgrade paths on testnet BEFORE mainnet.
- Verify oracles on testnet.

### Step 8: Upgrade strategy

| Pattern | When | Trade-offs |
|---------|------|------------|
| **Immutable** | Critical value; trust-minimised | No bug fixes; emergency requires migration |
| **Transparent Proxy** (EIP-1967) | Common; admin function separation | More complex; admin can't call implementation |
| **UUPS** (EIP-1822) | Cheaper deploy; logic in implementation | Risk of upgrade bricking if not careful |
| **Diamond** (EIP-2535) | Large contracts; facet splitting | Very complex; tooling immature |
| **Module-based** (Aptos/Sui) | Native upgrade; resource versioning | Newer model |
| **Governance-only** | DAO-managed | Slow; politically complex |

**Timelock**: every admin action (mint, pause, upgrade,
parameter change) MUST be behind a timelock (24-72h). Users see
the change coming and can exit if they disagree.

**Emergency pause**: critical functions (transfer, withdraw,
borrow) should be pausable. Multisig-controlled; documented
incident-response runbook.

### Step 9: Mainnet deployment

Pre-flight checklist:
- [ ] Audit complete; all critical/high findings remediated
- [ ] Bug bounty live
- [ ] Multisig (Safe) for all admin roles; key holders distributed
- [ ] Timelock on all admin actions
- [ ] Monitoring on value flows, oracle events, governance
- [ ] Emergency-pause procedure documented; tested
- [ ] Incident response runbook
- [ ] Verified contracts on Etherscan / block explorer
- [ ] Frontend integrates with verified contract addresses only
- [ ] Domain / DNS / frontend hosting security reviewed

Deployment:
1. Deploy implementation contract.
2. Deploy proxy pointing at implementation.
3. Initialize proxy with safe parameters.
4. Transfer admin role from deployer EOA to multisig (Safe).
5. Configure timelock with multisig as proposer.
6. Verify on block explorer.
7. Announce via official channels only.

### Step 10: Continuous monitoring

| Signal | Alert |
|--------|-------|
| TVL drop > X% in Y min | Investigate (could be hack, could be users exiting) |
| Oracle staleness > threshold | Pause operations depending on feed |
| Governance proposal submitted | Notify stakeholders; review |
| Large admin action queued in timelock | Verify intent; community review |
| Contract upgrade event | Verify against expected upgrade; alert |
| Unusual gas usage on functions | Could indicate exploit attempt |
| Repeated failed calls to admin | Could be attacker probing |
| Anomalous token approvals | User-side phishing? Contract issue? |

Tools: Forta, Tenderly, OpenZeppelin Defender, custom bots,
block-explorer notifications.

## Smart-contract gotchas

| Issue | Impact | Fix |
|-------|--------|-----|
| Re-entrancy | Drain contract | CEI pattern; ReentrancyGuard |
| Single-key admin | Compromised key → drained | Multisig + timelock |
| Unaudited contract with value | Public exploit | Audit; bug bounty |
| Stale oracle | Liquidations at wrong prices | Staleness threshold; multiple oracles |
| Unbounded loop | DoS via gas | Pagination; pull pattern |
| tx.origin auth | Phishing | msg.sender only |
| Selfdestruct in proxy | Funds locked or drained | No selfdestruct in any modern contract |
| Unprotected init | Front-run the initializer | `_disableInitializers` + `initializer` modifier |
| Storage collision (proxy) | State corruption | Use EIP-1967 slots; append-only storage |
| Front-running approval | Drained wallet | Permit (EIP-2612); explicit approval UX |

## Examples

### Example 1: DeFi lending protocol

```
Chain:        Ethereum L1 (with L2 expansion planned)
Value at risk: $50M initially; high after launch
Upgrade:      UUPS proxy + 48h timelock + 3/5 multisig
Oracle:       Chainlink (primary) + Uniswap TWAP (fallback)
Audit:        Trail of Bits + Spearbit; $500k bug bounty

Architecture:
  - Pool contract: holds liquidity; mints share tokens
  - Interest rate model: pluggable; per-asset
  - Liquidation engine: per-asset liquidator incentives
  - Price oracle: reads Chainlink; checks staleness + deviation
  - Treasury: protocol fees
  - Governance: token-weighted; timelock

Threats addressed:
  - Re-entrancy: ReentrancyGuard + CEI
  - Oracle manipulation: staleness check + TWAP fallback
  - Flash loan: reentrancy + per-block borrow caps
  - Governance: timelock; quorum; vote delegation
  - Liquidation front-running: MEV-aware design

Monitoring:
  - TVL anomaly detection
  - Oracle deviation alerts
  - Governance proposal notifier
  - Emergency pause on critical functions
```

### Example 2: NFT marketplace

```
Chain:        Base (L2) for low fees
Value at risk: $5M (royalties + escrow)
Upgrade:      Transparent proxy + 24h timelock + 2/3 multisig
Oracle:       none (royalty enforcement is on-chain)
Audit:        Single firm (Code4rena contest); $100k bounty

Architecture:
  - Marketplace contract: order matching; royalty enforcement
  - Escrow: holds NFTs during sale
  - Royalty registry: configurable per collection
  - Fee treasury: protocol fees

Threats addressed:
  - Royalty bypass: enforce on-chain; block low-fee marketplaces
  - Fake listings: signature verification on accept
  - Re-entrancy: CEI + guard
  - Front-running: commit-reveal for high-value listings

Monitoring:
  - Volume anomaly
  - Royalty bypass attempts
  - Large transfer events
```

### Example 3: DAO treasury management

```
Chain:        Ethereum L1
Value at risk: $200M
Upgrade:      Governance-only (token-weighted)
Oracle:       Chainlink for any external price dependency
Audit:        Multi-firm + formal verification on critical paths

Architecture:
  - Treasury: holds assets; only callable via governance
  - Governance: proposals; voting; timelock
  - Token: vote + quorum; delegation
  - Executor: queues and executes passed proposals

Threats addressed:
  - Governance attack: long voting period; quorum; emergency
    cancel by security council
  - Proposal front-running: vote snapshots; delegation
  - Flash loan governance: voting power snapshots at block N-1
  - Treasury drain: spending limits per period; multi-sig
    veto on critical ops
  - Upgrade bomb: timelock; security council; 7-day exit window

Monitoring:
  - Active proposals; voting participation
  - Treasury outflow alerts
  - Emergency pause by security council
```

## Anti-patterns

### ❌ Unaudited contract with value at risk

Every contract holding value needs an audit before mainnet.
Smart-contract bugs are public and immediate; exploits drain
treasuries in one transaction.

### ❌ Single-key admin

One compromised key drains the contract. Use multisig (Safe)
+ timelock for all admin actions. 3/5 minimum.

### ❌ Re-entrancy-unprotected external calls

Classic exploit; never disappears. Every external call needs
re-entrancy guard or checks-effects-interactions pattern.

### ❌ Single oracle

One price feed goes stale → liquidations at wrong prices. Use
multiple oracles (Chainlink + Uniswap TWAP); check staleness
and deviation.

### ❌ Upgradeable without timelock

Admin can upgrade silently; users have no time to exit.
24-72h timelock is the floor.

### ❌ Pre-0.8 Solidity without SafeMath

Integer overflow / underflow. Use 0.8+ with checked arithmetic
or SafeMath explicitly.

### ❌ "It's just a frontend bug"

Frontend bugs can drain wallets. Sign-phishing, malicious tx
params, fake approve popups. Treat frontend with the same care
as contracts.

## Failure modes

| Failure | Recovery |
|---------|----------|
| Exploit detected | Emergency pause; isolate affected contracts; engage audit firm; communicate |
| Oracle stale | Pause operations; switch to fallback; document |
| Admin key compromised | Revoke compromised key via timelock + governance; rotate; post-mortem |
| Governance attack | Emergency cancel via security council (if designed in); engage community |
| Frontend serves malicious tx params | Take frontend offline; publish warning; coordinate with wallets |
| Audit found critical bug post-deploy | Pause; deploy fix; coordinate migration; communicate |
| Upgrade bricked contract | Recover via governance / recovery pattern (if designed); honest post-mortem |
| Re-entrancy exploited | Pause; patch; redeploy via proxy; audit fix |

## Related skills

| Skill | When to chain |
|-------|---------------|
| `/kickoff` | Initial scoping (chain choice, value at risk) |
| `/roadmap` | Track audit cycles, upgrade timelines, oracle migrations |
| `/security` | Threat modelling, secret management, audit logging |
| `/api` | Off-chain API integration; indexer design |
| `/incident` | Severity ladder; on-call; emergency pause |