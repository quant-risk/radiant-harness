# Skill: game

> Game-development guidance: engine choice, game loop, asset pipeline,
> perf budgets, state management, multiplayer, platform certification.
> A game without a perf budget is a movie. A movie without state
> boundaries is a crash loop.

## Decision tree

```
Project starts (or pivots to game)
        │
        ▼
[Step 1] Engine + platform     ── decide BEFORE prototyping
        │
        ▼
[Step 2] Architecture doc      ── docs/game/architecture.md
        │
        ▼
[Step 3] Perf budget           ── docs/game/perf-budget.md
        │                          (frame, memory, asset, GC)
        ▼
[Step 4] Game loop             ── fixed timestep + interpolation
        │
        ▼
[Step 5] State machine         ── loading / menu / gameplay / paused
        │
        ▼
[Step 6] Asset pipeline        ── import, build, ship
        │
        ▼
[Step 7] Multiplayer design    ── docs/game/netcode-design.md (if needed)
        │
        ▼
[Step 8] Save/load             ── versioned, migratable
        │
        ▼
[Step 9] Release checklist     ── docs/game/release-checklist.md
                                   (per-platform certification)
```

## Workflow

### Step 1: Engine + platform

Engine choice cascades through everything — asset format, scripting
language, deployment pipeline, team skills, licensing cost.

| Engine | Strengths | Weaknesses | Best for |
|--------|-----------|------------|----------|
| **Unity** | C# + huge ecosystem; mobile + cross-platform | Memory overhead; IL2CPP quirks | Indie / AA / mobile-first |
| **Unreal** | Photoreal rendering; C++ + Blueprints | Heavy; high min-spec; long iteration | AAA / FPS / simulation |
| **Godot** | Lightweight; open-source; GDScript + C# | Smaller ecosystem; some platform gaps | Indie / 2D / web export |
| **Bevy** | Rust ECS; modern; data-oriented | Younger; less tooling; smaller community | Tools / simulation / tech demos |
| **Custom** | Full control | You write everything; ports are nightmares | Specialised (e.g. mobile hyper-casual) |

Don't switch engines mid-project. Lock it.

### Step 2: Architecture document

`docs/game/architecture.md` captures:

- Engine + version
- Target platforms + min-spec for each
- Game loop strategy (fixed timestep)
- State machine (top-level: loading / menu / gameplay / paused)
- Input abstraction (gamepad, keyboard, touch, motion)
- Asset pipeline (import → build → ship)
- Multiplayer architecture (if any): topology, tick rate,
  prediction model
- Save/load format + location (local / cloud)
- Engine subsystems owned vs replaced (physics, audio, networking,
  UI)
- Performance budgets (see Step 3)

### Step 3: Perf budget

`docs/game/perf-budget.md` per platform, locked BEFORE feature work.
Perf debt compounds; "we'll optimise later" never happens.

| Budget | Typical target (60fps PC) | Typical target (30fps mobile) |
|--------|--------------------------|------------------------------|
| Frame time | 16.6ms | 33.3ms |
| Draw calls | 1-3k | 100-300 |
| Triangles | 1-5M | 50-200k |
| SetPass calls | 200-500 | 30-80 |
| Total memory | 4-8GB | 200-500MB |
| Texture memory | 1-2GB | 50-150MB |
| GC alloc per frame | <1KB (steady-state) | <0.5KB |

Profile on **min-spec hardware**, not your dev box. Use Unity
Profiler, Unreal Insights, Tracy, or platform-native tools (RenderDoc,
Instruments, Snapdragon Profiler).

### Step 4: Game loop — fixed timestep with interpolation

The textbook pattern: **fixed simulation timestep, variable render
rate with interpolation**. See Glenn Fiedler, "Fix Your Timestep"
(gafferongames.com).

```
accumulator += dt
while accumulator >= fixedDt:
    simulate(fixedDt)
    accumulator -= fixedDt
alpha = accumulator / fixedDt
render(state_prev, state_curr, alpha)
```

Why:
- Deterministic physics: every system sees the same simulation
  tick regardless of frame rate.
- Replays work: input log + same initial state → same outcomes.
- Network sync is sane: clients can run fixed-timestep and
  reconcile.

Variable timestep without interpolation causes:
- Physics jitter at low frame rates
- Non-deterministic collisions (frame-rate-dependent bug)
- Replays that diverge from live gameplay
- Multiplayer desync

If you genuinely need variable timestep (e.g. real-time strategy
with thousands of units), you need a more sophisticated determinism
story — but most games don't.

### Step 5: State machine

Top-level game states must have **explicit transitions** with
**clear ownership of resources**:

```
[Boot]
  │ init engine, load persistent settings
  ▼
[Loading]
  │ async load level assets
  ▼
[Menu]            ◀──┐
  │ start game      │
  ▼                 │
[Gameplay] ─────pause┘
  │ player dies / level complete
  ▼
[GameOver] / [LevelComplete]
  │ retry / quit
  ▼
[Menu]
```

Common mistake: the "global state object" pattern. Every system
pokes it; ordering bugs arise; save/load becomes a freeze-the-world
snapshot; multiplayer is impossible.

Use an explicit state machine with:
- Clear transitions (UI/code that owns each transition)
- Resource ownership per state (loading owns the asset handles;
  gameplay does not)
- An `OnEnter`/`OnExit` hook for state-specific work
- A pause-aware simulation tick

### Step 6: Asset pipeline

The asset pipeline converts source assets (art files, audio,
fonts) into runtime formats:

1. **Author** in DCC tools (Blender, Maya, Photoshop, Aseprite)
2. **Import** via engine importer (may convert format, generate
   mipmaps, compress)
3. **Reference** in scenes/prefabs
4. **Build** into the final binary or asset bundles
5. **Ship** to platform

Pitfalls:
- **Source assets in the build** (uncompressed PSDs shipping in
  the build). Always cook through the import pipeline.
- **No version control for binary assets** (designer drops .blend
  files in Slack). Use Git LFS, Perforce, or Plastic SCM.
- **Asset build determinism** ("why is the build 50MB bigger
  today?"). Pin importer settings; reproducible builds.
- **No addressables / asset bundles** for downloadable content.
  Loading everything at boot is fine for small games; terrible
  for live-service.

### Step 7: Multiplayer design (if applicable)

`docs/game/netcode-design.md` if the game is multiplayer:

| Topology | When | Trade-offs |
|----------|------|------------|
| **P2P (lockstep)** | Small player counts (≤8), deterministic simulation | Cheapest infra; sensitive to one bad peer; one cheater breaks it |
| **P2P (host)** | Mid-count (≤16), one player is authoritative | Slightly more resilient; host advantage; host migration is hard |
| **Dedicated server** | Competitive, large player counts | Most authoritative; expensive to run; anti-cheat friendly |
| **Client-server with prediction** | Action games (FPS, fighting) | Smooth feel; reconciliation needed; rollback netcode |

Key choices:
- **Tick rate**: 30Hz minimum for shooters; 60Hz preferred; 128Hz
  for fighting games
- **State vs input sync**: send inputs and simulate both sides
  (bandwidth-cheap, requires determinism) OR send state (bandwidth-
  heavy, no determinism needed)
- **Lag compensation**: rewind the world to where the shooter
  saw the target when validating a hit (server-authoritative)
- **Reconciliation**: client predicts local movement; when the
  server corrects, snap or smooth-blend back

Don't ship multiplayer without a "what happens when the network
is bad" plan: 100ms latency, 5% packet loss, one peer dropping
mid-game.

### Step 8: Save / load

Versioned, migratable, secure (don't trust client):

- Header: `{ format_version: 3, game_version: "1.4.2",
  timestamp, ... }`
- Migration path: v1 → v2 → v3 transforms on load
- Local saves: user-writable directory (per-platform:
  `Application.persistentDataPath`, `%APPDATA%/Saved Games/`,
  cloud-saved if the platform supports it)
- Integrity: CRC or HMAC; reject tampered saves
- Autosave: timed + on-checkpoint

If you can load a save from version N-2 of your game, you've
shipped good saves. If saves break every release, players quit.

### Step 9: Release checklist

`docs/game/release-checklist.md` per platform:

**Console (Sony / Microsoft / Nintendo)**:
- TRC / XR / Lotcheck requirements (hundreds of items)
- Submission 4-6 weeks before launch (cert queues are long)
- Re-submission budget (typical: 2-3 cert cycles before pass)
- Region-specific builds (PAL vs NTSC, JP rating, etc.)
- Save data portability rules
- Trophies / achievements + parity across platforms

**Steam**:
- Store page assets (capsule, header, screenshots, trailer)
- Build uploaded + depots configured
- Steamworks SDK integration (achievements, leaderboards,
  cloud saves, DLC)
- Pricing + regional pricing strategy
- Beta branch + keys for QA

**Mobile (App Store / Play Store)**:
- See `/mobile` skill for full mobile checklist
- IARC rating
- Privacy nutrition labels (data collected, used for tracking)
- Subscription IAP rules (Apple's recent policy changes)

**PC (other stores)**: Epic, GOG, Itch.io, Humble — each has
its own submission rules.

## Game-specific gotchas

| Issue | Impact | Fix |
|-------|--------|-----|
| Frame drop on min-spec | Players refund / 1-star review | Profile min-spec from day 1; perf budget |
| Long loading screen | First impression is "this game is slow" | Async loading + progress UI; preload on splash |
| Save corruption | Players lose progress | Versioned saves + CRC + autosave on checkpoint |
| Multiplayer desync | "Why did I teleport back?" | Fixed timestep; reconciliation; lag compensation |
| Asset build size blew up | 4GB patch when 200MB expected | Asset audit; texture compression; bundle streams |
| Console cert failure | 4-week resubmission cycle | Read TRC/XR early; build a cert matrix into CI |
| Platform fragmentation | Game works on dev, crashes on PlayStation | Platform-specific testing; hardware lab |

## Examples

### Example 1: 2D indie platformer (PC + Switch)

```
Engine:    Godot 4.2 (GDScript)
Platform:  PC (Steam) + Nintendo Switch
Scale:     indie (3 devs)
Target FPS: 60

Architecture:
  - Game loop: fixed timestep (1/60s), render interpolation
  - State: Boot → Loading → Menu → Gameplay → Pause → GameOver
  - Input: gamepad + keyboard abstraction; rebindable
  - Asset pipeline: import → .pck bundles; streaming optional
  - Multiplayer: local couch (2P), shared screen

Perf budget (PC + Switch docked):
  - Frame time: 16.6ms / draw calls ≤300 / GC alloc ≈0
  - Texture memory: ≤200MB / total memory: ≤1GB

Save format:
  - JSON + CRC, versioned (v3), autosave on level complete
  - 3 save slots + autosave slot

Release:
  - Steam: capsule, trailer, achievements, cloud saves
  - Switch: Lotcheck submission 6 weeks before launch
  - Region builds: NA / EU / JP
```

### Example 2: AA multiplayer shooter (PC + console)

```
Engine:    Unreal 5.4 (C++ + Blueprints for designers)
Platform:  PC (Steam) + PS5 + Xbox Series
Scale:     AA (25 devs)
Target FPS: 60 (120 on PC high-end)

Architecture:
  - Game loop: fixed 60Hz simulation; render interp
  - State: explicit; gameplay owns world state
  - Input: per-platform abstraction; aim-assist on consoles
  - Multiplayer: dedicated server (32-player), 60Hz tick
  - Netcode: client-side prediction + server reconciliation

Perf budget (PS5 baseline):
  - Frame time: 16.6ms (90% headroom for next-gen features)
  - Draw calls: 2-3k; triangles: 2-4M
  - Memory: ≤10GB total

Save format:
  - Profile (settings, unlocks) + per-match state (server-side)
  - No local save of competitive match state

Release:
  - Cert cycles: 2 Sony + 2 Microsoft before launch
  - Cross-play: PS5 + Xbox + PC via EOS or first-party
  - Anti-cheat: kernel-mode (EAC or BattlEye)
```

### Example 3: mobile hyper-casual (iOS + Android)

```
Engine:    Unity 2022 LTS (C#)
Platform:  iOS + Android
Scale:     small team (5 devs)
Target FPS: 60 (30 minimum on low-end)

Architecture:
  - Game loop: fixed timestep, but render rate adapts
  - State: single scene, state-machine on root controller
  - Asset pipeline: addressables for streaming
  - Multiplayer: none (leaderboards via Game Center / Play
    Games Services)

Perf budget (mid-range Android):
  - Frame time: 33ms / draw calls ≤80 / GC ≈0
  - Memory: ≤250MB

Save format:
  - PlayerPrefs (best score, settings); cloud save via
    platform SDK

Release:
  - App Store + Play Store; A/B test icons + screenshots
  - Mediation for ads (AdMob + Unity Ads + Meta)
  - ASO keywords; localisation for top 10 markets
```

## Anti-patterns

### ❌ Variable timestep without interpolation

Non-deterministic physics, jitter, replay divergence, multiplayer
desync. Use fixed timestep with render interpolation. (Glenn
Fiedler's pattern is canonical.)

### ❌ Loading assets in the gameplay frame

Causes visible hitches. Async load before the gameplay state
enters; show a loading screen with progress.

### ❌ "We'll optimise later"

Perf debt compounds. Lock a perf budget before features and
measure against it every PR. Otherwise the last 20% of perf
work takes 80% of the schedule.

### ❌ Optimising on the dev machine

Your dev box is not your players' box. Profile on min-spec
hardware (the device, the GPU, the memory budget) from day 1.

### ❌ Single global state object

Creates ordering bugs, breaks save/load, breaks multiplayer.
Use explicit state machines with clear ownership of resources.

### ❌ Skipping platform cert pre-work

Console cert (Sony TRC, MS XR, Nintendo Lotcheck) is hundreds of
items. Reading them on day 1 of cert week is too late. Build a
cert-matrix into CI.

### ❌ Saves without versions

A save format change without migration breaks every existing
player's progress. Version + migration is the only way.

## Failure modes

| Failure | Recovery |
|---------|----------|
| Frame rate drops below target | Profile; check draw calls / overdraw / GC; ship hotfix |
| Cert failure (console) | Read the full failure report; fix; resubmit; budget 4-6 weeks per cycle |
| Save corruption in the wild | Pull the save format; add CRC + version; ship recovery tool |
| Multiplayer desync reports | Add diagnostic logs; record client+server state; reproduce; fix determinism gap |
| Asset build blew up | Audit recent asset imports; pin importer settings; verify deterministic build |
| Engine upgrade breaks the project | Pin engine version; run a sandbox upgrade project; only merge if all tests pass |

## Related skills

| Skill | When to chain |
|-------|---------------|
| `/kickoff` | Initial engine + platform decision (uses game inputs) |
| `/roadmap` | Track platform porting, engine upgrade timelines |
| `/mobile` | When mobile is a target (deeper mobile-specific guidance) |
| `/incident` | Cert failure; mass user-reported crash; save loss |
| `/setup-ci` | Build a CI matrix per platform (cert gates, asset builds) |
| `/security` | Anti-cheat; save integrity; client-trust boundaries |