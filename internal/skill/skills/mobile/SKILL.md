# Skill: mobile

> Mobile-first guidance for shipping iOS / Android / cross-platform apps.
> App store, offline-first, OS fragmentation — mobile is not web.

## Decision tree

```
Project starts (or pivots to mobile)
        │
        ▼
[Step 1] Platform decision
        │
        ├── iOS only? → Swift + UIKit / SwiftUI
        ├── Android only? → Kotlin + Jetpack Compose
        ├── Both, native? → 2 codebases (most performant)
        ├── Both, cross? → Flutter / RN / KMP (1 codebase, some perf cost)
        └── PWA? → web skill applies (not this skill)
        │
        ▼
[Step 2] Offline strategy
        │
        ├── Online-only (e.g. social feed, news)
        │   └── Backend is source of truth; cache optional
        ├── Offline-first (e.g. notes, todo)
        │   └── Local DB is source of truth; sync via CRDT or queue
        └── Eventual consistency (e.g. messaging)
            └── Local-first writes; sync on reconnect
        │
        ▼
[Step 3] Auth method
        │
        ├── OAuth (Google / Apple / Facebook) — common, but adds SDK
        ├── Biometric — convenient, but fallback to PIN needed
        ├── Magic link — simple, but requires email open
        └── Passkey (WebAuthn) — modern, but adoption varies
        │
        ▼
[Step 4] Architecture: docs/mobile/architecture.md
        │
        ▼
[Step 5] Release checklist: docs/mobile/release-checklist.md
```

## Workflow

### Step 1: Platform decision

The platform choice cascades through everything:

| Decision | Implication |
|----------|-------------|
| iOS only | Swift + Xcode; App Store review (avg 24h, can reject) |
| Android only | Kotlin + Android Studio; Play Console (faster, hours) |
| Both native | 2 codebases, 2 teams OR 2x cost for one team |
| Flutter | 1 codebase, 2 platforms; some perf hit for heavy UIs |
| React Native | 1 codebase, 2 platforms; JS ecosystem; bridge overhead |
| KMP | Shared Kotlin logic, native UI; emerging |

Don't change platform mid-project. The decision is durable.

### Step 2: Offline strategy

Decide BEFORE coding. The offline strategy determines the data
layer:

| Strategy | Data layer | Sync |
|----------|-----------|------|
| Online-only | Server is truth; local cache is best-effort | None |
| Offline-first | Local DB (SQLite, Realm, Isar) is truth | CRDT or outbox queue |
| Eventual | Local writes; sync on reconnect | Server merges by timestamp |

Common mistakes:
- Building offline-first as an afterthought → rewrite the data layer
- Using `online-only` for a tool users want on planes / subways
- Using `offline-first` for content that's always-fresh (news feed)

### Step 3: Auth

Auth choice depends on user demographics and security needs:

| Method | Pros | Cons |
|--------|------|------|
| OAuth (Google/Apple) | Familiar, fast signup | Requires SDK, account fragmentation |
| Biometric (Face ID, fingerprint) | Fast, no password | Needs fallback (PIN); device-dependent |
| Magic link | Simple, no password | Requires email access; phishable |
| Passkey (WebAuthn) | Phishing-resistant, modern | Adoption still uneven; complex setup |

For most consumer apps: Apple/Google Sign-In + biometric unlock.
For enterprise: passkey or SSO.

### Step 4: Architecture decisions

Document in `docs/mobile/architecture.md`:

- Platform: <ios/android/cross>
- Min OS version: <e.g. iOS 16 / Android 8>
- Offline strategy: <online-only/offline-first/eventual>
- Auth: <oauth/biometric/magic-link/passkey>
- Push notifications: <APNs / FCM / both>
- Analytics: <Firebase / Amplitude / Mixpanel / self-hosted>
- Crash reporting: <Crashlytics / Sentry / Bugsnag>
- CI/CD: <Fastlane / Bitrise / GitHub Actions>

### Step 5: Release checklist

`docs/mobile/release-checklist.md` (per release):

**Apple App Store**:
- [ ] App icon (1024x1024 PNG, no transparency)
- [ ] Screenshots for each device class (6.5", 5.5", iPad)
- [ ] Description (4000 char max)
- [ ] What's new (4000 char max, for updates)
- [ ] Keywords (100 char max)
- [ ] Privacy policy URL (must be live, HTTPS)
- [ ] Age rating questionnaire completed
- [ ] App Review information filled out
- [ ] Export compliance (encryption usage)
- [ ] TestFlight internal + external tested
- [ ] Version + build number bumped

**Google Play Console**:
- [ ] App icon (512x512 PNG)
- [ ] Feature graphic (1024x500)
- [ ] Screenshots (phone + tablet, 7" and 10")
- [ ] Short description (80 char)
- [ ] Full description (4000 char)
- [ ] Privacy policy URL
- [ ] Content rating (IARC questionnaire)
- [ ] Target audience + content
- [ ] Data safety form completed
- [ ] Internal test + closed track + open track tested
- [ ] Version code + name bumped

## Mobile-specific gotchas

| Issue | Impact | Fix |
|-------|--------|-----|
| App Store review rejection | 1-7 day delay | Submit early; use TestFlight/Internal Track |
| Android device fragmentation | Crashes on old devices | Test on min OS + 2 below; Firebase Test Lab |
| iOS background restrictions | Background tasks killed | Use BGTaskScheduler; foreground services |
| Network unreliability | Lost writes | Outbox pattern; retry with backoff |
| OS permission changes (e.g. ATT) | Tracking broken | Plan for opt-in rates <30% |
| Cold start time | Users abandon | Lazy init; measure with Firebase Performance |
| Memory pressure | OOM crashes on low-end devices | Profile with Instruments / Android Profiler |

## Examples

### Example 1: consumer fitness app (iOS-first)

```
Platform: iOS only (Swift + SwiftUI)
Min OS: iOS 16 (covers 92% of active devices)
Offline: offline-first (workouts tracked offline, sync when online)
Auth: Apple Sign-In + biometric unlock
Push: APNs only (FCM would add 50ms latency on iOS)
Analytics: Firebase + Amplitude
Crash: Crashlytics
CI/CD: GitHub Actions + Fastlane
```

### Example 2: cross-platform team productivity tool

```
Platform: cross (Flutter, 1 codebase, 2 platforms)
Min OS: iOS 15 / Android 8 (covers 95% combined)
Offline: eventual consistency (local DB writes, server merges)
Auth: OAuth (Google for Android, Apple for iOS)
Push: FCM (cross-platform unified)
Analytics: Firebase
Crash: Sentry
CI/CD: Codemagic
```

## Anti-patterns

### ❌ No offline strategy

"Don't worry about offline, our users always have signal" — until
they don't. Plan for offline-first or explicit "online required"
UX from day 1.

### ❌ App Store metadata last

Submitting to the App Store with broken screenshots, missing
privacy policy, or wrong age rating means a 1-7 day delay. Build
the metadata draft in parallel with the code.

### ❌ Cross-platform for "performance"

If your app is heavy on animations / games / AR, native beats
cross-platform. Choose Flutter/RN for productivity tools, not for
graphics-heavy apps.

### ❌ Skipping min OS analysis

Picking iOS 14 "for broader reach" without checking distribution
data means supporting devices that have known perf issues. Always
check Apple/Google's distribution stats.

## Failure modes

| Failure | Recovery |
|---------|----------|
| App Store rejection | Fix metadata; resubmit; budget 24-48h for re-review |
| Offline sync conflicts | Last-write-wins by default; escalate to CRDT if users complain |
| Crash spike after release | Roll back via phased rollout (Play Store) or expedite review (App Store) |
| Push notifications throttled by OS | Move to silent push for sync; only use alert push for user-initiated events |

## Related skills

| Skill | When to chain |
|-------|---------------|
| `/kickoff` | Initial architecture decision (uses mobile inputs) |
| `/roadmap` | Track platform migration, OS upgrade timelines |
| `/incident` | App Store rejection; mass user-reported crash; sync outage |