# Skill: iot

> Connected-device architecture, firmware, connectivity, OTA,
> security at the edge, power management. IoT runs in the real
> world: power fails, networks drop, sensors lie, devices brick.

## Decision tree

```
Project starts (physical device + connectivity)
        │
        ▼
[Step 1] Device class           ── sensor / gateway / appliance / wearable
        │
        ▼
[Step 2] MCU + connectivity     ── trade-offs (power, cost, RF)
        │
        ▼
[Step 3] Architecture           ── docs/iot/architecture.md
        │                          (hardware, firmware, OTA, cloud)
        ▼
[Step 4] Power budget           ── docs/iot/power-budget.md
        │                          (measured on hardware)
        ▼
[Step 5] Security model         ── docs/iot/security-model.md
        │                          (secure boot, keys, attestation)
        ▼
[Step 6] Firmware build         ── toolchain, RTOS, libraries
        │
        ▼
[Step 7] OTA strategy           ── docs/iot/firmware-update-plan.md
        │                          (signed images, staged, rollback)
        ▼
[Step 8] Cloud integration      ── telemetry, control, OTA channel
        │
        ▼
[Step 9] Failure-mode testing   ── power loss, dropout, brick recovery
        │
        ▼
[Step 10] Pilot + scale-up
```

## Workflow

### Step 1: Device class

| Class | Example | Resources | Connectivity |
|-------|---------|-----------|--------------|
| **Sensor node** | Temp sensor, leak detector | MCU (KB RAM); battery | BLE / LoRa / Zigbee |
| **Edge gateway** | Industrial hub, smart-home hub | Linux SoC; 100s MB RAM | Wi-Fi / Ethernet |
| **Appliance** | Smart fridge, EV charger | Linux SoC; multi-GB RAM | Wi-Fi / Ethernet |
| **Wearable** | Fitness tracker, smart watch | MCU (low-power); battery | BLE |
| **Automotive** | Telematics, ADAS | MCU + SoC; safety-critical | CAN / Cellular |

Device class drives every downstream choice: power, connectivity,
compute budget, security model, OTA strategy.

### Step 2: MCU + connectivity

| MCU family | Strengths | Best for |
|-----------|-----------|----------|
| **ESP32 / ESP32-S3** | Cheap; Wi-Fi + BLE; mature | Wi-Fi connected sensors |
| **STM32 (L0/L4/H7)** | Low power; broad range; real-time | Industrial, automotive |
| **nRF52 / nRF53** | Ultra-low power; BLE; Thread/Matter | Wearables, smart home |
| **RP2040** | Cheap; dual-core; Pico ecosystem | Hobbyists, prototypes |
| **Linux SoC (RPi, NXP i.MX, Allwinner)** | Full Linux; rich tooling | Gateways, appliances |
| **Arduino (AVR/ARM)** | Hobbyist; mature; slow | Education, prototypes |

Connectivity trade-offs:

| Tech | Range | Bandwidth | Power | Best for |
|------|-------|-----------|-------|----------|
| **BLE 5.x** | ~50m | 2 Mbps | Low | Phone-paired; wearables |
| **Wi-Fi** | ~50m | 100+ Mbps | High | Always-connected appliances |
| **LoRa** | ~10km | <50 kbps | Very low | Wide-area sensors; agriculture |
| **Zigbee / Thread / Matter** | ~30m mesh | 250 kbps | Low | Smart home mesh |
| **Cellular (LTE-M / NB-IoT)** | Anywhere | <1 Mbps | Moderate | Asset tracking; remote |
| **Ethernet** | Wired | 1 Gbps | Low | Industrial; gateways |

### Step 3: Architecture

`docs/iot/architecture.md`:

- **Hardware**: MCU + sensors + actuators + power + connectivity
  module (block diagram)
- **Firmware architecture**: RTOS vs bare-metal; module layout;
  driver / app / OTA / cloud-client separation
- **Boot sequence**: ROM bootloader → secure boot → application
- **State machine**: boot → provisioning → operational →
  OTA-pending → OTA-updating → recovery
- **Cloud integration**: telemetry protocol (MQTT, HTTPS);
  OTA channel; control plane
- **Update strategy**: A/B partitions; recovery; rollback
- **Provisioning**: factory keys; per-device certificates;
  on-boarding flow

Signed off BEFORE prototype. The decisions here are expensive
to change later (NRE is sunk; tooling is sunk; supply chain is
locked).

### Step 4: Power budget

`docs/iot/power-budget.md`:

| Mode | Current draw | Duration | mAh per day |
|------|--------------|----------|-------------|
| Active (sensor read + transmit) | 80 mA | 5 ms × 200/day | 0.022 |
| Sleep (deep) | 10 µA | 23.99 h | 0.24 |
| **Total** | — | — | **~0.26 mAh/day** |

Battery life = battery_capacity / daily_consumption.

| Battery | Capacity (mAh) | Life (days) at 0.26 mAh/d |
|---------|---------------|----------------------------|
| CR2032 coin cell | 220 | 846 |
| 2x AA alkaline | 2500 | 9615 |
| LiPo 1000 mAh | 1000 | 3846 |

**Validate on hardware**. Estimating from datasheets misses:
- Wi-Fi connection overhead (seconds, not ms)
- Sleep-mode current (often higher than datasheet min)
- Voltage regulator quiescent current
- Sensor warm-up time

Use a power profiler (Nordic Power Profiler Kit, Joulescope,
Otii Arc) on real hardware.

### Step 5: Security model

`docs/iot/security-model.md`:

| Concern | Mitigation |
|---------|------------|
| **Firmware tampering** | Secure boot with signed images; verify signature before boot |
| **Physical flash access** | Encrypted flash (Secure Element, eFuse); disable JTAG in production |
| **Key theft** | Per-device keys; secure provisioning at factory; never shared |
| **Man-in-the-middle** | TLS 1.3 with certificate pinning; mutual TLS for cloud |
| **Replay attacks** | Nonces in requests; signed timestamps |
| **OTA hijack** | Signed firmware images; verification BEFORE flashing |
| **Side-channel** | Constant-time crypto; masking (for high-security) |
| **Privacy** | Don't send PII unnecessarily; encrypt at rest and in transit |

Provisioning:

- **Factory**: per-device key generated in HSM; injected during
  manufacturing; certificate chain tied to manufacturer CA.
- **Onboarding**: per-device attestation (TPM / SE) to prove
  device identity to cloud; cloud issues per-device cert.
- **Rotation**: keys rotated periodically; old keys gracefully
  expire; new keys provisioned OTA.

### Step 6: Firmware build

Toolchain:

| Component | Choices |
|-----------|---------|
| **Toolchain** | arm-none-eabi-gcc (ARM); ESP-IDF (ESP32); platformio (cross); Zephyr (RTOS) |
| **RTOS** | FreeRTOS; Zephyr; NuttX; ThreadX; bare-metal |
| **Language** | C (most); C++ (some); Rust (emerging); MicroPython (prototyping) |
| **Build** | CMake + Ninja; Make; platformio |
| **Test** | Unity / CMock (unit); Ceedling (test runner); QEMU (simulation) |
| **Static analysis** | cppcheck; clang-tidy; Coverity |
| **SBOM** | CycloneDX for embedded; SPDX |

RTOS or bare-metal:
- **RTOS**: easier concurrency; more memory; better for complex
  apps (sensor + connectivity + OTA + UI)
- **Bare-metal**: smaller; lower power; harder to write; best for
  simple sensor nodes

### Step 7: OTA updates

`docs/iot/firmware-update-plan.md`:

**Image layout (A/B partitions)**:

```
Flash layout:
  [0x0000]  Bootloader          (signed)
  [0x1000]  Partition Table
  [0x2000]  Slot A (active)     (signed)
  [0x... ]  Slot B (staging)     (signed)
  [0xN000]  NVS / config
```

**Update flow**:

1. Cloud pushes new image to device (signed, encrypted in transit).
2. Device verifies signature; writes to inactive slot.
3. Device marks inactive slot as "ready to swap".
4. Device reboots into bootloader.
5. Bootloader swaps active slot; boots from new image.
6. New image runs; if it crashes within watchdog window,
   bootloader rolls back to previous slot.
7. Old slot marked "abandoned" after grace period.

**Recovery from brick**:

- **A/B + watchdog rollback**: automatic recovery.
- **External programmer** (JTAG/SWD): requires physical access;
  expensive in the field.
- **Factory reset button**: triggers bootloader mode for
  re-provisioning.

**Update cadence**:
- **Security patches**: within 7 days of disclosure.
- **Bug fixes**: monthly.
- **Features**: quarterly.

For pilot (< 10k): manual rollout acceptable.
For scale (10k-1M): staged rollout (1% → 10% → 50% → 100%).
For mass (>1M): fleet management with cohorts, canary, abort.

### Step 8: Cloud integration

| Concern | Choice |
|---------|--------|
| **Telemetry transport** | MQTT (lightweight, pub/sub); HTTPS POST (simple) |
| **Schema** | Protobuf (typed, efficient); CBOR; JSON (debug) |
| **Auth** | Mutual TLS with per-device certs; JWT with refresh |
| **OTA channel** | Same MQTT broker or separate HTTPS endpoint |
| **Data pipeline** | Ingest → queue → stream processor → time-series DB |
| **Fleet management** | Per-device shadow; desired vs reported state; commands |
| **Observability** | Per-device metrics; fleet-level aggregates |

MQTT is the IoT standard: lightweight, pub/sub, retained
messages, QoS levels. Tools: HiveMQ, EMQX, AWS IoT Core,
Azure IoT Hub.

### Step 9: Failure-mode testing

Test on hardware, not in simulation:

| Failure | Expected behaviour |
|---------|-------------------|
| Power loss mid-write | Recover to last known good state; no corruption |
| Wi-Fi dropout mid-transmission | Retry with backoff; queue locally |
| Battery removed | Device restarts cleanly when power returns |
| Sensor disconnected | Log error; don't hang; degrade gracefully |
| OTA interrupted (power loss) | Bootloader recovers; complete or rollback |
| Watchdog timer fires | Device reboots; state preserved |
| Time set backwards | Detect; resync; log |
| Cloud unreachable | Buffer locally; retry; back off |
| Provisioning repeated (same device) | Idempotent; no double-provisioning |
| Re-flash with old firmware | Secure boot rejects (if min-version enforced) |

### Step 10: Pilot → scale

| Stage | Volume | Focus |
|-------|--------|-------|
| **Prototype** | < 100 | Architecture validation; design choices |
| **Pilot** | 100 - 10k | Field reliability; OTA validation; support load |
| **Scale** | 10k - 1M | Fleet management; monitoring; cost per device |
| **Mass** | > 1M | Supply chain; manufacturing QA; long-term support |

Each stage introduces new failure modes. Don't skip.

## IoT-specific gotchas

| Issue | Impact | Fix |
|-------|--------|-----|
| Estimated power budget | 10x worse than expected | Measure on hardware |
| Untested OTA | Bricked devices in field | Test exhaustively on hardware |
| Shared device keys | One leak = full fleet compromise | Per-device keys; HSM provisioning |
| No watchdog | Hung devices; field calls | Hardware watchdog; timer resets MCU |
| Secrets in flash | Stolen via JTAG / SWD | Encrypted flash; Secure Element |
| Wi-Fi connection slow | Battery drained | Connection caching; long sleep; event-driven |
| Time not synced | Logs out of order; TLS fails | NTP / RTC sync; resync on reconnect |
| Unsigned OTA | Anyone can flash malicious firmware | Signed images; verify before boot |
| No rollback path | Bad update = recall | A/B partitions; watchdog rollback |

## Examples

### Example 1: battery temp sensor (LoRa)

```
Device class: sensor-node
MCU:          STM32L0 (ARM Cortex-M0+, 192KB flash, 20KB RAM)
Connectivity: LoRa (SX1276)
Power:        2x AA alkaline (2500 mAh)
Volume:       pilot (5k units)

Architecture:
  - Bare-metal (no RTOS; simple state machine)
  - Sleep most of the time; wake every 15 min to read + transmit
  - LoRaWAN with OTAA activation; per-device DevEUI + AppKey
  - Factory-provisioned keys (HSM); per-device certs not needed
    for LoRaWAN

Power budget (measured):
  - Active: 80 mA for 3s, every 15 min
  - Sleep: 8 µA otherwise
  - Daily: 0.25 mAh/day
  - Battery life: 10,000 days (~27 years) — derated to 5 years

OTA:
  - Single-bank flash; no A/B (cost-sensitive)
  - Firmware update via LoRaWAN downlink (slow but works)
  - Recovery: physical button triggers bootloader + factory reset
  - Cadence: security patches only (no feature updates)
```

### Example 2: smart home hub (Linux SoC + Matter)

```
Device class: edge-gateway
MCU:          NXP i.MX 8M (Linux SoC, 4GB RAM)
Connectivity: Wi-Fi + Ethernet; Thread border router; BLE
Power:        mains (USB-C PD)
Volume:       scale (200k units)

Architecture:
  - Yocto Linux; systemd; Docker for app containers
  - Local control plane; cloud sync optional
  - Matter / Thread border router for smart-home devices
  - OTA via A/B partitions; signed images
  - Per-device cert issued by manufacturer CA

Power budget:
  - Mains; no battery concern
  - Idle: 3W; peak: 8W

Security:
  - Secure boot (NXP HAB); encrypted storage (LUKS)
  - Per-device TLS cert; rotated via OTA
  - Hardware root of trust (TPM 2.0)
  - Matter attestation certificates

OTA:
  - A/B partitions; staged rollout (1% → 10% → 100%)
  - Image signed by manufacturer; verified in bootloader
  - Auto-rollback on watchdog / boot failure
  - Cadence: monthly (features); within 7 days (security)

Failure modes:
  - Power loss: filesystem journaled; clean recovery
  - Wi-Fi dropout: local control continues; cloud sync retries
  - Update interrupted: rollback to previous slot on next boot
  - Disk full: log rotation; auto-cleanup
```

### Example 3: industrial vibration sensor (cellular)

```
Device class: sensor-node (industrial)
MCU:          nRF52840 (BLE) + LTE-M modem (u-blox SARA-R4)
Connectivity: cellular (LTE-M)
Power:        battery + solar harvest (10W panel)
Volume:       pilot (1k units)

Architecture:
  - nRF52840 sensor hub (vibration, temp)
  - u-blox LTE-M modem for backhaul
  - Solar MPPT charger; battery backup
  - Cloud: AWS IoT Core (MQTT + shadow)

Power budget:
  - Sampling: 50 mA for 1s, every 5 min (vibration analysis)
  - Modem active: 200 mA for 30s, every hour (transmit)
  - Sleep: 50 µA otherwise
  - Solar harvest (10W panel, 4h sun avg): ~4000 mAh/day
  - Net daily: -3.5 mAh (surplus)

Security:
  - Per-device cert (factory-provisioned)
  - TLS 1.3 to AWS IoT
  - Signed firmware; A/B partitions; OTA via cellular

Failure modes:
  - Cellular dropout: buffer locally (1MB flash); retry on reconnect
  - Solar insufficient: battery lasts 7 days at average draw
  - OTA over cellular: small delta updates; bandwidth-aware
  - Watchdog: nRF watchdog + cellular modem watchdog
```

## Anti-patterns

### ❌ Estimating power budget by calculation

Datasheet numbers are typical, not worst-case. Wi-Fi connection
takes seconds, not ms. Sleep current is higher than min. Measure
on hardware with a power profiler.

### ❌ Untested OTA

Bricking 1,000 devices is a recall. Bricking 1M is company-
ending. Test the OTA path exhaustively on hardware BEFORE pilot.

### ❌ Shared device keys

One leak compromises the entire fleet. Per-device keys,
factory-provisioned via HSM, per-device attestation.

### ❌ "We'll add security later"

Physical devices are accessible. Serial ports are exposed.
Flash can be read. Secure boot + encrypted storage + signed
firmware from day 1.

### ❌ Skipping watchdog / brownout detection

Embedded devices hang. Without a watchdog timer, a hung sensor
is a field-call. Hardware watchdog resets the MCU if firmware
fails to pet it.

### ❌ Storing secrets in flash without encryption

Flash can be read via JTAG / SWD. Secrets must be in encrypted
storage (Secure Element, eFuse, or encrypted flash with keys
in SE).

### ❌ Ignoring duty cycle / latency budget

"Send data every 10ms" on a battery device = 1-week battery.
Real-time is expensive. Quantify the duty cycle and its impact
on battery life BEFORE designing the firmware.

## Failure modes

| Failure | Recovery |
|---------|----------|
| OTA bricked device | A/B rollback via bootloader; field tech reflashes |
| Power loss mid-write | Journaling filesystem; state preserved |
| Watchdog fires | Reboot; restore from NVS; report incident |
| Cellular dropout | Buffer locally; retry with backoff |
| Sensor disconnected | Log error; degrade; alert cloud |
| Cloud unreachable | Buffer telemetry; sync on reconnect |
| Battery low | Reduce duty cycle; enter low-power mode |
| Secure boot rejection | Reflash via factory programmer; investigate |
| Time desync | NTP resync; flag events as untrusted |

## Related skills

| Skill | When to chain |
|-------|---------------|
| `/kickoff` | Initial device + MCU scoping |
| `/roadmap` | Track MCU lifecycle, connectivity standards (Matter) |
| `/security` | Threat modelling, key provisioning, audit logging |
| `/api` | Cloud API for telemetry, control, OTA |
| `/mobile` | Companion mobile app for device setup + control |
| `/data` | Telemetry pipeline; time-series; anomaly detection |