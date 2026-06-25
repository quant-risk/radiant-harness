# Skill: data

> Data engineering: pipelines, warehouses, analytics.
> Lineage matters. Schema evolution is hard. The pipeline IS the product.

## Decision tree

```
Project starts (or pivots to data)
        │
        ▼
[Step 1] Architecture decision
        │
        ├── Lakehouse (S3 + Iceberg/Delta + Spark/Trino)
        │   └── Best for: raw + curated in one place; ML workloads
        ├── Warehouse (Snowflake/BigQuery/Redshift + dbt)
        │   └── Best for: SQL-first teams; structured analytics
        ├── Stream (Kafka/Pulsar + Flink/Spark Streaming)
        │   └── Best for: real-time; event-driven; sub-second latency
        └── Mesh (decentralized ownership per domain)
            └── Best for: large orgs; multiple teams owning their data
        │
        ▼
[Step 2] Source systems
        │
        ├── OLTP DB (postgres, mysql)
        ├── SaaS APIs (salesforce, stripe, hubspot)
        ├── Events (app logs, clickstream)
        ├── Files (S3, GCS, on-prem)
        └── External (partners, public datasets)
        │
        ▼
[Step 3] Schema evolution strategy
        │
        ├── Expand-and-contract (recommended)
        │   1. Add nullable new column
        │   2. Backfill old data
        │   3. Switch readers
        │   4. Drop old column
        ├── Migration (acceptable for small schemas)
        │   └── atomic schema change; downtime required
        └── Dual-write (acceptable for high-stakes changes)
            └── write both old + new; switch reads; deprecate old
        │
        ▼
[Step 4] Architecture: docs/data/architecture.md
        │
        ▼
[Step 5] Quality checks: docs/data/quality-checks.md
```

## Workflow

### Step 1: Architecture decision

The architecture choice cascades through everything:

| Architecture | Storage | Compute | Best for |
|--------------|---------|---------|----------|
| Lakehouse | S3 + Iceberg/Delta | Spark, Trino | ML + analytics in one place |
| Warehouse | Snowflake, BigQuery | SQL, dbt | SQL-first teams |
| Stream | Kafka, Pulsar | Flink, Spark Streaming | Real-time, event-driven |
| Mesh | Per-domain warehouse | dbt per domain | Large orgs, multiple teams |

**Recommendation for most teams**: start with a warehouse (BigQuery /
Snowflake / Redshift) and dbt. Lakehouse if you need ML. Stream
only if you genuinely need sub-second latency.

### Step 2: Source systems

Identify every source the data team will read from. Common
sources:
- **OLTP DB**: change data capture (CDC) via Debezium / Fivetran
- **SaaS APIs**: Fivetran / Airbyte / Stitch / custom
- **Events**: app logs, clickstream (Kafka, Kinesis, Pub/Sub)
- **Files**: S3 / GCS with CSV / JSON / Parquet
- **External**: partners, public datasets, SEC filings

For each source, document:
- Connector used (and its latency SLA)
- Schema (and how it changes — do they version it?)
- Volume (rows/day or events/sec)
- Criticality (P0 = drop everything, P3 = nice to have)

### Step 3: Schema evolution

Pick ONE strategy and document it. Schema evolution is the
#1 cause of data corruption in pipelines.

**Expand-and-contract** (recommended):

```
Day 1:  orders(id INT, total DECIMAL)        -- readers use `total`
Day 2:  orders(id INT, total DECIMAL, total_cents BIGINT NULL)  -- readers still use `total`
Day 3:  backfill `total_cents` from `total * 100`  -- historical data populated
Day 4:  writers write BOTH `total` and `total_cents`
Day 5:  readers switch to `total_cents` (gradual; one query at a time)
Day 6:  writers stop writing `total`
Day 7:  drop `total` column
```

Never do an in-place migration of a column that's actively being
read AND written. You WILL corrupt data.

### Step 4: Architecture document

`docs/data/architecture.md`:

- **Sources**: list with connectors + SLAs
- **Storage**: where data lands at each stage (raw → staging → mart)
- **Compute**: which tool runs which transformation
- **Orchestration**: Airflow / Dagster / Prefect / cron
- **Observability**: what alerts fire when a pipeline breaks
- **Lineage**: OpenLineage / DataHub / Marquez
- **Cost**: Snowflake credits / BigQuery slots / S3 storage
- **PII handling**: where PII lives; how it's masked

### Step 5: Quality checks

`docs/data/quality-checks.md` per dataset:

| Check | Type | Threshold | Action on fail |
|-------|------|-----------|----------------|
| Row count | volume | > 90% of expected | Page on-call |
| Null count in `user_id` | completeness | 0 | Page on-call |
| Distinct `event_type` values | uniqueness | = expected set | Slack alert |
| `created_at` max freshness | freshness | < 4h old | Page on-call |
| Foreign key to `users.id` | referential | 0 orphans | Page on-call |
| Sum of `amount` matches ledger | reconciliation | exact match | Page finance |

Tools: Great Expectations, Soda Core, dbt tests, Monte Carlo.

## Examples

### Example 1: SaaS analytics pipeline (warehouse)

```
Sources: postgres (CDC), stripe (Fivetran), segment (events)
Architecture: Snowflake + dbt
Orchestration: dbt Cloud
Lineage: dbt's manifest.json → DataHub
Quality: dbt tests (unique, not_null, relationships, custom)
Cost: ~$2k/mo Snowflake; ~$500/mo Fivetran
```

### Example 2: ML feature store (lakehouse)

```
Sources: events (Kafka), postgres (CDC), external CSVs
Architecture: S3 + Iceberg + Spark + Feast (feature store)
Orchestration: Airflow
Lineage: Marquez
Quality: Great Expectations on Iceberg tables
Cost: ~$5k/mo S3 + EMR
```

## Anti-patterns

### ❌ "Schema-on-read, we'll figure it out"

Schema-on-read is great for exploration, terrible for production.
In production, you need contracts: every column has a type, every
field has an owner, every change has a notice.

### ❌ In-place migrations

`ALTER TABLE orders MODIFY COLUMN total BIGINT NOT NULL` while the
table is being read + written = data corruption. Always expand-and-contract.

### ❌ No data tests

If your pipeline produces `null` for `user_id` and nobody notices
for 3 weeks, you've shipped bad data. Tests catch it.

### ❌ Single point of failure

One Airflow instance, one Snowflake account, one engineer who knows
the schema. The bus factor is 1. Document + automate + cross-train.

## Failure modes

| Failure | Recovery |
|---------|----------|
| Pipeline breaks silently (no alert) | Add row-count + freshness checks; alert on fail |
| Schema change breaks downstream | Expand-and-contract protocol; never in-place |
| Source system change unnoticed | Source schema diff in CI; alert on drift |
| Cost spike (warehouse runaway) | Cost budgets per pipeline; alert at 80% |

## Related skills

| Skill | When to chain |
|-------|---------------|
| `/kickoff` | Initial architecture decision (uses data inputs) |
| `/roadmap` | Track pipeline migrations, deprecations |
| `/incident` | Broken pipeline affecting dashboards / ML models |