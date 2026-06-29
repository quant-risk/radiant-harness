# Machine Learning Pipelines — Production Patterns from Real Repos

## 1. Overview

This skill codifies the **10 universal patterns** extracted from analysis of 10 production ML repositories: made-with-ml, Metaflow, Kedro, TFX, ZenML, Flyte, DVC, Evidently, Feast, and Seldon Core.

**When to use:** Building reproducible, versioned, monitored production ML pipelines — any domain.

**Key insight:** No single framework does everything well, but the patterns are universal and composable. Extract the patterns, not the frameworks.

---

## 2. Top 10 Production ML Patterns

### Pattern 1: Decorator-Based Pipeline Definition
**Sources:** Metaflow, ZenML, Flyte, made-with-ml

Construct DAGs from pure Python — no YAML, no DSL. Each decorated function becomes a pipeline node.

**Metaflow — `FlowSpec` + `@step` + `self.next()`** (from `metaflow/flowspec.py`):
```python
from metaflow import FlowSpec, step, Parameter, kubernetes, timeout, retry

class TrainingFlow(FlowSpec):
    n_trials = Parameter("n_trials", default=50)

    @step
    def start(self):
        self.next(self.load_data)

    @step
    @kubernetes(cpu=4, memory=16000)
    @timeout(minutes=30)
    @retry(times=2)
    def load_data(self):
        import pandas as pd
        self.df = pd.read_parquet("s3://data-lake/training/")
        self.next(self.train)

    @step
    def train(self):
        self.model = fit_model(self.df)
        self.next(self.end)

    @step
    def end(self):
        print("Pipeline complete")

if __name__ == "__main__":
    TrainingFlow()
```

**ZenML — `@step`/`@pipeline` with caching** (from `zenml/steps/step_decorator.py`):
```python
from zenml import step, pipeline

@step(enable_cache=True, experiment_tracker="mlflow_tracker")
def train_step(X_train, y_train, config: dict):
    import xgboost as xgb
    model = xgb.XGBClassifier(**config, tree_method="hist")
    model.fit(X_train, y_train)
    return model

@pipeline(enable_cache=True)
def training_pipeline():
    raw = load_data()
    X_train, X_test, y_train, y_test = split_data(raw)
    model = train_step(X_train, y_train, {"n_estimators": 500})
    evaluate(model, X_test, y_test)
```

---

### Pattern 2: Data Catalog / Registry
**Sources:** Kedro (`DataCatalog` + `_LazyDataset`), Feast, TFX

Central registry with lazy materialization, type-safe access, version management. Separates data access from computation.

**Kedro — Lazy dataset materialization** (from `kedro/io/data_catalog.py`):
```yaml
# catalog.yml — data access decoupled from code
raw_data:
  type: pandas.CSVDataset
  filepath: data/01_raw/training.csv
model:
  type: pickle.PickleDataset
  filepath: data/06_models/model.pkl
  versioned: true
```

```python
from kedro.pipeline import Pipeline, node

def create_pipeline(**kwargs):
    return Pipeline([
        node(train_model, inputs=["X_train", "y_train", "params:model"],
             outputs="trained_model"),
        node(evaluate_model, inputs=["trained_model", "X_test", "y_test"],
             outputs="metrics"),
    ])
```

Change from CSV to Parquet by editing YAML — zero code changes.

---

### Pattern 3: Component / Flavor Abstraction
**Sources:** ZenML (Stack/Flavor), TFX (Spec/Executor/Driver), Kedro (`AbstractDataset`), Seldon (CRDs)

Pluggable implementations for orchestrators, artifact stores, deployers. Infrastructure-agnostic code.

**TFX — Spec/Executor/Driver pattern:**
```python
class TrainerSpec(types.ComponentSpec):
    INPUTS = {
        'examples': ComponentParameter(type=standard_artifacts.Examples),
        'schema': ComponentParameter(type=standard_artifacts.Schema),
    }
    OUTPUTS = {
        'model': ComponentParameter(type=standard_artifacts.Model),
    }
    EXECUTOR_CLASS = TrainerExecutor
```

Swap orchestrators (local → Kubeflow), artifact stores (local → S3), deployers (local → Seldon) without changing pipeline code.

---

### Pattern 4: Lifecycle Hooks
**Sources:** Kedro, Metaflow, ZenML

Cross-cutting concerns — logging, metrics, validation — without modifying pipeline code.

```python
# Kedro hook pattern
class MetricLoggingHook:
    @hook_impl
    def after_node_run(self, node, inputs, outputs):
        if "metrics" in outputs:
            mlflow.log_metrics(outputs["metrics"])

    @hook_impl
    def on_pipeline_error(self, error, run_params):
        send_alert(f"Pipeline failed: {error}")
```

---

### Pattern 5: Config Layering
**Sources:** Kedro (OmegaConf), Metaflow, DVC (`params.yaml`), ZenML

Base config → environment overrides → CLI overrides → runtime parameters.

**DVC — params.yaml + dvc.yaml:**
```yaml
# params.yaml
train:
  n_estimators: 500
  learning_rate: 0.05

# dvc.yaml
stages:
  train:
    cmd: python train.py
    deps: [data/, train.py]
    params: [train.n_estimators, train.learning_rate]
    outs: [model.pkl]
    metrics: [metrics.json]
```

**Kedro — YAML + OmegaConf:**
```yaml
# conf/base/parameters.yml (defaults)
model_params:
  n_estimators: 500
# conf/local/parameters.yml (overrides, gitignored)
model_params:
  n_estimators: 1000
```

---

### Pattern 6: Point-in-Time Correct Feature Retrieval
**Sources:** Feast, TFX Transform

Prevent data leakage: features computed only from data available at prediction time.

**Feast** (from `feast/feature_store.py`, `feast/feature_view.py`):
```python
from feast import Entity, FeatureView, Field, FeatureStore
from feast.types import Float32
from datetime import timedelta

customer = Entity(name="customer_id", join_keys=["customer_id"])
features = FeatureView(
    name="customer_features", entities=[customer],
    schema=[Field(name="balance", dtype=Float32),
            Field(name="consistency", dtype=Float32)],
    source=FileSource(path="data/features.parquet", timestamp_field="event_timestamp"),
    ttl=timedelta(days=1),
)

store = FeatureStore(repo_path=".")
# Training: point-in-time joins prevent leakage
training_df = store.get_historical_features(
    entity_df=entity_df_with_timestamps,
    features=["customer_features:balance"],
).to_df()
# Serving: low-latency online
online_df = store.get_online_features(
    features=["customer_features:balance"],
    entity_rows=[{"customer_id": "C001"}],
).to_df()
```

---

### Pattern 7: Artifact Versioning + Lineage
**Sources:** DVC, Metaflow, TFX (MLMD), ZenML

Every artifact automatically versioned with provenance.

**DVC** (from `dvc/dvcfile.py`):
```bash
dvc stage add -n train -d data/ -o model.pkl -M metrics.json python train.py
dvc repro          # re-executes only changed stages
dvc exp run         # experiment tracking via Git branches
dvc exp show        # compare experiments
dvc push            # artifacts to remote storage
```

**Metaflow:** Every `self.*` artifact auto-versioned in datastore. Access past artifacts: `Run('TrainingFlow/1234').data.model`.

---

### Pattern 8: Typed I/O Between Components
**Sources:** TFX, Kedro, ZenML, Flyte

Steps declare typed inputs/outputs. Framework validates connections at build time.

```python
# Kedro: "model" output from train must match "model" input to evaluate
pipeline = Pipeline([
    node(train_model, inputs=["X_train", "y_train"], outputs="model"),
    node(evaluate_model, inputs=["model", "X_test", "y_test"], outputs="metrics"),
    # Mismatched types → error at construction, not after 30min training
])
```

---

### Pattern 9: Smart Caching / Reproducibility
**Sources:** DVC, Metaflow, Kedro, ZenML

Cache intermediate results; only re-execute when inputs change.

```bash
# DVC: only changed stages re-run
dvc repro
# Output: Stage 'train' didn't change, skipping
```

```python
# ZenML: step-level cache control
@step(enable_cache=True)
def preprocess(data): return heavy_transform(data)

@step(enable_cache=False)  # always re-run
def train(data): return fit(data)
```

---

### Pattern 10: Declarative Deployment
**Sources:** Seldon Core, Feast, ZenML, Flyte

Deployment as configuration, not imperative scripts.

**Seldon Core — K8s CRDs** (from `operator/apis/mlops/v1alpha1/model_types.go`):
```yaml
apiVersion: mlops.seldon.io/v1alpha1
kind: Model
metadata:
  name: classifier-v2
spec:
  storageUri: "s3://models/classifier/v2"
  requirements: [sklearn]
  memory: "1Gi"
---
apiVersion: mlops.seldon.io/v1alpha1
kind: Experiment
metadata:
  name: ab-test
spec:
  default: classifier-v1
  candidates:
    - name: classifier-v1
      weight: 90
    - name: classifier-v2
      weight: 10
```

---

## 3. Pattern Matrix

| Pattern | made-with-ml | metaflow | kedro | tfx | zenml | flyte | dvc | evidently | feast | seldon |
|---|---|---|---|---|---|---|---|---|---|---|
| Decorator Pipelines | ✓ | ✓✓ | – | – | ✓✓ | ✓✓ | – | – | – | – |
| Data Catalog | – | – | ✓✓ | ✓ | ✓ | – | – | – | ✓✓ | – |
| Component Abstraction | – | ✓ | ✓✓ | ✓✓ | ✓✓ | ✓ | – | – | ✓ | ✓✓ |
| Lifecycle Hooks | – | ✓ | ✓✓ | – | ✓ | – | – | – | – | – |
| Config Layering | – | ✓ | ✓✓ | – | ✓ | – | ✓✓ | – | ✓ | ✓ |
| Point-in-Time | – | – | – | – | – | – | – | – | ✓✓ | – |
| Artifact Versioning | ✓ | ✓✓ | ✓ | ✓✓ | ✓ | – | ✓✓ | – | – | – |
| Typed I/O | ✓ | – | ✓✓ | ✓✓ | ✓ | ✓ | – | ✓ | – | ✓ |
| Smart Caching | – | ✓ | ✓ | – | ✓ | – | ✓✓ | – | – | – |
| Declarative Deploy | – | – | – | – | ✓ | ✓ | – | – | ✓✓ | ✓✓ |

> ✓✓ = primary strength, ✓ = present, – = not a focus

---

## 4. Evaluation & Monitoring

**Evidently — Report/Metric pattern** (from `evidently/core/report.py`):
```python
from evidently import Report
from evidently.presets import DataDriftPreset
from evidently.core.datasets import DataDefinition, Dataset

definition = DataDefinition(
    numerical_columns=["income", "balance"],
    categorical_columns=["segment"],
    target="target",
)
ref_ds = Dataset.from_pandas(reference_df, data_definition=definition)
cur_ds = Dataset.from_pandas(current_df, data_definition=definition)
report = Report(metrics=[DataDriftPreset()], include_tests=True)
result = report.run(reference_data=ref_ds, current_data=cur_ds)
result.save_html("reports/drift.html")
```

**Slice-based evaluation** (from `made-with-ml/madewithml/evaluate.py`):
```python
from snorkel.slicing import PandasSFApplier, slicing_function

@slicing_function()
def high_value(x):
    return x.value_score > 0.8

@slicing_function()
def short_text(x):
    return len(x.text.split()) < 8

def get_slice_metrics(y_true, y_pred, df):
    slices = PandasSFApplier([high_value, short_text]).apply(df)
    results = {}
    for name in slices.dtype.names:
        mask = slices[name].astype(bool)
        if sum(mask):
            p, r, f, _ = precision_recall_fscore_support(
                y_true[mask], y_pred[mask], average="micro")
            results[name] = {"precision": p, "recall": r, "f1": f}
    return results
```

---

## 5. Serving

**Ray Serve + FastAPI** (from `made-with-ml/serve.py`):
```python
from fastapi import FastAPI
from ray import serve

app = FastAPI(title="ML API")

@serve.deployment(num_replicas="1", ray_actor_options={"num_cpus": 8})
@serve.ingress(app)
class ModelDeployment:
    def __init__(self, run_id: str, threshold: float = 0.9):
        self.threshold = threshold
        self.predictor = load_predictor(run_id)

    @app.post("/predict/")
    async def predict(self, request):
        data = await request.json()
        results = self.predictor.predict(data)
        for r in results:
            if r["probability"] < self.threshold:
                r["prediction"] = "uncertain"
        return {"results": results}

    @app.get("/health/")
    def health(self): return {"status": "healthy"}
```

---

## 6. Recommended Stack

| Layer | Recommended | Alternative | Pattern |
|---|---|---|---|
| Orchestration | Metaflow | ZenML | Decorator Pipelines |
| Data Management | Kedro DataCatalog | DVC | Catalog + Versioning |
| Feature Store | Feast | – | Point-in-Time |
| Experiment Tracking | MLflow | – | Integrates with all |
| Evaluation | Evidently | Snorkel slices | Report/Metric |
| Serving | Seldon Core (K8s) | Ray Serve | Declarative Deploy |
| Monitoring | Evidently + Prometheus | – | Drift Detection |

---

## 7. Common Pitfalls

1. **Data leakage via temporal splits.** Use Feast `get_historical_features()` for point-in-time correctness.
2. **Feature engineering before split.** Compute inside CV loop or use TFX Transform.
3. **No config layering.** Use Kedro OmegaConf or DVC params.yaml — no hardcoded params.
4. **Missing artifact versioning.** Use DVC `dvc repro` or Metaflow's auto-versioned datastore.
5. **Imperative deployment.** Use Seldon CRDs, Feast `apply`, or ZenML stacks instead of bash scripts.
6. **No slice evaluation.** Overall metrics hide subpopulation failures. Use Snorkel or Evidently descriptors.
7. **Untyped connections.** Use Kedro typed nodes or TFX artifact types to catch mismatches at build time.
8. **Training-serving skew.** Validate feature parity with canary datasets. Use TFX Transform.
9. **No caching.** Use DVC `dvc repro` or ZenML `enable_cache=True` for iteration speed.
10. **Monitoring without alerts.** Evidently reports need threshold triggers (PSI > 0.01, AUC drop > 2%).

---

## 8. Verification Checklist

- [ ] Pipeline uses decorators (Metaflow/ZenML/Flyte), not imperative scripts
- [ ] Data in catalog (Kedro DataCatalog) with versioning
- [ ] Config layered: base → env → CLI → runtime
- [ ] Features use point-in-time joins (Feast) — no temporal leakage
- [ ] Artifacts auto-versioned (DVC/Metaflow/MLflow)
- [ ] Typed I/O connections validated at build time
- [ ] Smart caching enabled — only changed stages re-execute
- [ ] Evaluation includes slice metrics, not just overall
- [ ] Deployment is declarative (CRDs/apply/stacks)
- [ ] Drift monitoring with automated alerts configured
- [ ] Lifecycle hooks for logging, metrics, error handling
- [ ] Rollback strategy: previous model tagged and redeployable

---

## 9. Implementation Internals

Deep architecture patterns extracted from source-level reading of Metaflow, Kedro, ZenML, and DVC.

### Metaflow's Metaclass System

`FlowSpecMeta` is the metaclass behind all Metaflow flows. On class creation, it builds a `_FlowState(MutableMapping)` that merges inherited and self-owned data with custom rules: dicts merge recursively, lists concatenate, and certain keys (CONFIGS, CACHED_PARAMETERS) stay local. The `FlowGraph` is constructed at class-definition time, not runtime.

```python
class FlowSpecMeta(type):
    def __init__(cls, name, bases, attrs):
        cls._flow_state = _FlowState({...})  # per-class mutable state
        cls._graph = FlowGraph(cls)           # DAG built at definition time
        cls._steps = [getattr(cls, node.name) for node in cls._graph]
```

Decorators (`StepDecorator`, `FlowDecorator`) register themselves via a registry pattern and provide ~20 lifecycle hooks (`step_init`, `task_pre_step`, `task_decorate`, `task_step_completed`, etc.). The execution model in `task.py` uses a **stack-based decorator execution** — wrappers execute `pre_step` in reverse order and `post_step` in forward order, with exception propagation through the stack.

### Kedro's `AbstractDataset.__init_subclass__` Auto-Wrapping

Every `AbstractDataset` subclass automatically gets error handling, logging, and init-arg capture — not by explicit code in each subclass, but via a single `__init_subclass__` hook:

```python
class AbstractDataset(abc.ABC, Generic[_DI, _DO]):
    def __init_subclass__(cls, **kwargs):
        # 1. Wraps __init__ to capture _init_args via inspect.getcallargs
        # 2. Aliases _load/_save as load/save if defined
        # 3. Wraps load/save with _load_wrapper/_save_wrapper for error handling
```

This means every dataset implementation automatically gets: exception wrapping (any error → `DatasetError`), logging on load/save, `None`-save prevention, and init argument capture for `_init_config()` serialization. The trade-off is fragility — it breaks if subclasses use `*args`/`**kwargs` in non-standard ways.

### Kedro's Pipeline Algebra

`Pipeline` supports algebraic composition via Python operators, enabling set-like pipeline manipulation:

```python
combined = pipeline_a + pipeline_b    # union of nodes
difference = pipeline_a - pipeline_b  # nodes in A but not B
intersection = pipeline_a & pipeline_b  # nodes in both
parallel = pipeline_a | pipeline_b    # parallel execution (no data deps)
```

Internally, `Pipeline` uses `graphlib.TopologicalSorter` for cycle detection at construction time, and validates namespace continuity across DAG paths via `_validate_namespaces()`.

### ZenML's StackComponent Secret Resolution

`StackComponentConfig` (Pydantic v2 BaseModel) overrides `__getattribute__` to intercept **all** attribute access. When an attribute contains a `{{secret_name.key}}` reference, it transparently resolves the secret at runtime:

```python
class StackComponentConfig(BaseModel, ABC):
    def __custom_getattribute__(self, name):
        value = super().__getattribute__(name)
        if isinstance(value, str) and re.match(r"\{\{.*\}\}", value):
            return resolve_secret_reference(value)
        return value
```

This makes secret injection invisible to step code — any config field can reference a secret without changing the consuming code. The cost is per-attribute-access overhead on every `StackComponentConfig` instance.

### DVC's StageCache (Content-Addressable Run Caching)

DVC's `StageCache` stores stage execution results in `.dvc/cache/runs/`. The cache key is a hash of `(command, dependency_paths_with_hashes)`. The value is the hash of the full lockfile content. On `dvc repro`, if the key matches, DVC restores cached outputs without re-running the command.

```python
class StageCache:
    def save(self, stage)       # Cache stage outputs by content hash
    def restore(self, stage)    # Restore if deps haven't changed
    def transfer(self, from_odb, to_odb)  # Move cache between remotes
```

Corrupted cache files are handled gracefully — `restore()` catches YAML parse errors and unlinks the corrupted entry.

### DVC's Re-Entrant Repo Locking

DVC uses re-entrant locking with depth counting for thread-safe repo operations:

```python
@contextmanager
def lock_repo(repo):
    # Depth counting: first acquire opens lock, nested calls increment counter
    # Resets graph cache on both acquire and release
    # The @locked decorator wraps repo methods with this context manager
```

Additionally, `@rwlocked` provides read/write locking on specific stage resources (deps, outs), and `@unlocked_repo` temporarily releases the lock during long-running command execution to allow concurrent access.

---

## 10. Hidden Gems

Lesser-known but powerful features found during deep source analysis.

### Metaflow's SpinRuntime

`SpinRuntime` enables fast local re-execution of individual steps using artifacts from a previous run's datastore. Instead of replaying the entire DAG, it reads artifacts from the original `FlowDataStore` and creates new tasks in a fresh run. Ideal for development iteration — re-run one step with code changes while reusing all upstream artifacts.

### Kedro's `graphlib.TopologicalSorter` for Cycle Detection

Kedro uses Python's stdlib `graphlib.TopologicalSorter` (not a custom implementation) to detect cycles at `Pipeline` construction time. This catches circular dependencies immediately rather than at execution time, with zero external dependencies.

### ZenML's Auto-Registration of Materializers via Metaclass

`BaseMaterializerMeta` (a metaclass) automatically registers every `BaseMaterializer` subclass in a global `materializer_registry` at class-definition time. The registry maps Python types to materializer classes, enabling automatic artifact serialization without explicit registration:

```python
class BaseMaterializerMeta(type):
    def __new__(mcs, name, bases, dct):
        cls = super().__new__(mcs, name, bases, dct)
        # Auto-registers ASSOCIATED_TYPES → cls in materializer_registry
        # SKIP_REGISTRATION flag for abstract base classes
```

### DVC's Comment-Preserving YAML Round-Trips

DVC uses `ruamel.yaml` with `parse_yaml_for_update()` + `apply_diff()` to modify `dvc.yaml` files while preserving comments, formatting, and ordering. When a stage is updated, only the changed lines are replaced — user comments in the YAML survive `dvc repro` cycles.

---

## 11. Anti-Patterns Found

Patterns observed in production codebases that serve as cautionary examples.

### Metaflow: NativeRuntime God Class

`NativeRuntime` in `metaflow/runtime.py` is **2485+ lines** managing tasks, workers, polling, cloning, resume, and foreach execution in a single class. It handles `_active_tasks`, `_run_queue`, `_workers` (fd → subprocess mapping), and uses `procpoll` for I/O multiplexing. This should be decomposed into separate TaskManager, WorkerPool, and ResumeManager classes.

### ZenML: Constructor Parameter Explosion

`BaseStep.__init__` accepts **30+ parameters** including `enable_cache`, `enable_artifact_metadata`, `experiment_tracker`, `step_operator`, `parameters`, `output_materializers`, `environment`, `secrets`, `settings`, `extra`, `on_failure`, `on_success`, `model`, `retry`, `cache_policy`, `runtime`, `heartbeat_healthy_threshold`, and more. `Pipeline.__init__` has 25+ similar parameters. This makes the API surface overwhelming and suggests the need for builder pattern or config objects.

### DVC: Repo Imports Operations as Class Attributes

DVC's `Repo` class imports **20+ functions as class attributes** from separate modules:

```python
class Repo:
    from dvc.repo.add import add
    from dvc.repo.checkout import checkout
    from dvc.repo.reproduce import reproduce
    from dvc.repo.push import push
    from dvc.repo.pull import pull
    # ... 20+ more
```

This keeps each operation in its own file (good) but makes `Repo` look like it has hundreds of methods (bad). IDE refactoring tools, type checkers, and documentation generators struggle with this pattern. A service-object or command pattern would be more conventional.


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

- `reinforcement-learning-pipelines`
- `causal-ml-inference`
- `econometrics-pipelines`

