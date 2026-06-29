# Context Engineering Skill

Deep architectural patterns for building RAG pipelines, vector stores, memory systems, and evaluation frameworks — distilled from production codebases (LlamaIndex, ChromaDB, Mem0, Langfuse, Ragas).

## Overview

Context Engineering covers RAG pipeline architecture, vector store design, memory systems, context compression, token efficiency, and evaluation metrics. Extracted from deep analysis of LlamaIndex, Mem0, ChromaDB, Langfuse, and Ragas.

**When to use**: Building, debugging, or optimizing systems in this domain.

## 1. RAG Pipeline Architecture

### 1.1 The Index → Retrieve → Synthesize Pipeline

The canonical RAG flow follows three stages with clear abstraction boundaries:

```
Documents → [Transform] → Nodes → [Index] → IndexStruct
                                                  ↓
Query → [Retrieve] → NodesWithScore → [Synthesize] → Response
```

**LlamaIndex Core Abstractions** (from `llama_index/core/`):

| Abstraction | Role | Key Interface |
|---|---|---|
| `BaseIndex[IS]` | Orchestrates document ingestion and index construction | `from_documents()`, `as_retriever()`, `as_query_engine()` |
| `BaseRetriever` | Fetches relevant nodes for a query | `_retrieve(query_bundle) → List[NodeWithScore]` |
| `BaseQueryEngine` | Full query pipeline (retrieve + synthesize) | `_query(query_bundle) → RESPONSE_TYPE` |
| `TransformComponent` | Document-to-node transformations (parsing, chunking) | `__call__(nodes) → nodes` |
| `StorageContext` | Persistence layer (docstore, index_store, vector_store, graph_store) | Composition root |

**Hidden Gem — Recursive Retrieval**: `BaseRetriever._handle_recursive_retrieval()` resolves `IndexNode` objects that point to sub-retrievers or sub-query-engines, enabling hierarchical index structures (e.g., document summary → document retriever).

```python
# Recursive retrieval pattern
class BaseRetriever:
    def _handle_recursive_retrieval(self, query_bundle, nodes):
        retrieved_nodes = []
        for n in nodes:
            node = n.node
            if isinstance(node, IndexNode):
                obj = node.obj or self.object_map.get(node.index_id)
                if obj is not None:
                    # Recurse into sub-retriever or sub-query-engine
                    retrieved_nodes.extend(
                        self._retrieve_from_object(obj, query_bundle, n.score)
                    )
            else:
                retrieved_nodes.append(n)
        return retrieved_nodes
```

### 1.2 Index Types and When to Use Each

| Index Type | Best For | Retrieval Strategy |
|---|---|---|
| `VectorStoreIndex` | Semantic similarity search | KNN/ANN on embeddings |
| `ListIndex` | Small docs, exhaustive context | Sequential scan |
| `KeywordTableIndex` | Keyword-based retrieval | TF-IDF / keyword extraction |
| `KnowledgeGraphIndex` | Entity-relationship queries | Graph traversal + vector |
| `TreeIndex` | Hierarchical summarization | Leaf-to-root traversal |
| `DocumentSummaryIndex` | Multi-document routing | Summary matching → doc retrieval |
| `PropertyGraphIndex` | Rich structured knowledge | Property graph queries |

### 1.3 Vector Store Query Modes

From `llama_index/core/vector_stores/types.py`:

```python
class VectorStoreQueryMode(str, Enum):
    DEFAULT = "default"          # Pure vector similarity
    SPARSE = "sparse"            # Sparse embeddings (BM25, SPLADE)
    HYBRID = "hybrid"            # Dense + sparse fusion
    TEXT_SEARCH = "text_search"  # Full-text search
    SEMANTIC_HYBRID = "semantic_hybrid"
    SVM = "svm"                  # SVM-based retrieval
    MMR = "mmr"                  # Maximum Marginal Relevance
```

**Anti-Pattern**: Using `DEFAULT` mode for all queries. Hybrid search (dense + sparse) consistently outperforms pure vector search for domain-specific corpora.

### 1.4 Metadata Filtering

LlamaIndex supports rich metadata filters with composable conditions:

```python
from llama_index.core.vector_stores import (
    MetadataFilter, MetadataFilters, FilterOperator, FilterCondition
)

filters = MetadataFilters(
    filters=[
        MetadataFilter(key="category", value="science", operator=FilterOperator.EQ),
        MetadataFilter(key="year", value=2024, operator=FilterOperator.GTE),
    ],
    condition=FilterCondition.AND,
)
```

Operators: `==`, `!=`, `>`, `<`, `>=`, `<=`, `in`, `nin`, `any`, `all`, `text_match`, `contains`, `is_empty`.

---

## 2. Vector Store Architecture (ChromaDB)

### 2.1 Core Type System

ChromaDB uses a **segment-based architecture** with three scopes:

```python
class SegmentScope(Enum):
    VECTOR = "VECTOR"      # Dense vector storage (HNSW index)
    METADATA = "METADATA"  # Metadata storage and filtering
    RECORD = "RECORD"      # Raw record storage
```

**Key Types** (from `chromadb/types.py`):

| Type | Purpose |
|---|---|
| `Collection` | Top-level container with id, name, configuration, tenant, database |
| `Segment` | Storage unit within a collection (vector, metadata, or record) |
| `OperationRecord` | Mutation request (ADD/UPDATE/UPSERT/DELETE) with embedding + metadata |
| `VectorQuery` | KNN/ANN query with k, allowed_ids, include_embeddings |
| `VectorQueryResult` | Result with id, distance, optional embedding |
| `LogRecord` | WAL entry with log_offset + OperationRecord |

### 2.2 Client Architecture

The `Client` class (from `chromadb/api/client.py`) follows a **singleton-per-resource** pattern:

```python
class Client(SharedSystemClient, ClientAPI):
    """Multiple clients connecting to the same resource share the same API instance."""
    tenant: str = DEFAULT_TENANT
    database: str = DEFAULT_DATABASE
    _server: ServerAPI       # Singleton server instance
    _admin_client: AdminAPI  # For tenant/database validation
```

**Hidden Gem**: `SharedSystemClient` manages reference counting. If init fails after refcount was incremented, it releases references to prevent resource leaks.

### 2.3 Collection Configuration

Collections store their configuration as JSON (`configuration_json`) and deserialize on access:

```python
class Collection(BaseModel):
    id: UUID
    name: str
    configuration_json: Dict[str, Any]  # Serialized CollectionConfiguration
    serialized_schema: Optional[Dict[str, Any]]
    metadata: Optional[Dict[str, Any]]
    dimension: Optional[int]
    tenant: str
    database: str
    version: int           # Always 0 in single-node; used in distributed
    log_position: int      # WAL position for distributed consistency
```

**Anti-Pattern**: Storing sensitive data in collection metadata — it's returned to all clients.

### 2.4 Embedding Function Registry

ChromaDB uses JSON schemas to validate and register embedding functions:

```
schemas/embedding_functions/
├── base_schema.json          # Base validation schema
├── openai.json               # OpenAI embedding config
├── ollama.json               # Ollama embedding config
├── huggingface.json          # HuggingFace embedding config
└── ...                       # 30+ providers
```

Each schema defines required/optional fields, defaults, and validation rules.

---

## 3. Memory Systems (Mem0)

### 3.1 Architecture Overview

Mem0 implements a **dual-layer memory system**: vector store for semantic search + SQLite for history tracking.

```
Memory.add(messages)
  ↓
[LLM] → Extract facts → Decide ADD/UPDATE/DELETE
  ↓
[Embedder] → Generate embeddings
  ↓
[VectorStore] → Store/search memories
  ↓
[SQLite] → Track history (what changed, when)
```

### 3.2 Factory Pattern for Providers

Mem0 uses four factories (from `mem0/utils/factory.py`):

```python
class LlmFactory:        # 18 providers: openai, anthropic, gemini, ollama, etc.
class EmbedderFactory:   # 11 providers: openai, huggingface, ollama, etc.
class VectorStoreFactory: # 24 providers: qdrant, chroma, pgvector, milvus, etc.
class RerankerFactory:   # 5 providers: cohere, sentence_transformer, etc.
```

**Hidden Gem**: `load_class()` uses `importlib.import_module` for lazy loading — providers are only imported when actually used, keeping startup fast.

```python
def load_class(class_type):
    module_path, class_name = class_type.rsplit(".", 1)
    module = importlib.import_module(module_path)
    return getattr(module, class_name)
```

### 3.3 Vector Store Base Interface

```python
class VectorStoreBase(ABC):
    @abstractmethod
    def create_col(self, name, vector_size, distance): ...
    @abstractmethod
    def insert(self, vectors, payloads=None, ids=None): ...
    @abstractmethod
    def search(self, query, vectors, top_k=5, filters=None): ...
    @abstractmethod
    def delete(self, vector_id): ...
    @abstractmethod
    def update(self, vector_id, vector=None, payload=None): ...
    @abstractmethod
    def get(self, vector_id): ...
    @abstractmethod
    def list(self, filters=None, top_k=None): ...
    @abstractmethod
    def reset(self): ...
    
    def keyword_search(self, query, top_k=5, filters=None):
        """Optional BM25 search. Returns None if not supported."""
        return None
    
    def search_batch(self, queries, vectors_list, top_k=1, filters=None):
        """Default: sequential. Override for native batch (e.g., Qdrant)."""
        return [self.search(q, v, top_k=top_k, filters=filters) 
                for q, v in zip(queries, vectors_list)]
```

**Score Normalization Contract**: All implementations must return similarity scores where higher = more similar (range [0, 1]):
- Cosine distance: `score = max(0.0, 1.0 - distance)`
- L2 distance: `score = 1.0 / (1.0 + distance)`
- Inner product: `score = value` (already higher = better)

### 3.4 Memory Addition Pipeline

The `add()` method in `mem0/memory/main.py` follows this pipeline:

1. **Validate & normalize** messages, metadata, entity IDs
2. **Build filters** from user_id/agent_id/run_id (at least one required)
3. **Route** to agent memory extraction (if agent_id + assistant messages) or user memory extraction
4. **LLM extraction** → structured facts from conversation
5. **Deduplication** → search existing memories, decide ADD/UPDATE/DELETE
6. **Vector operations** → upsert into vector store
7. **Entity linking** → extract entities, upsert into entity store
8. **History tracking** → record changes in SQLite

### 3.5 Entity Store Pattern

Mem0 maintains a separate vector collection for entities linked to memories:

```python
# Entity collection naming
entity_collection = f"{collection_name}_entities"  # or "-entities" for s3_vectors

# Entity payload
{
    "data": "Albert Einstein",
    "entity_type": "PERSON",
    "linked_memory_ids": ["mem-uuid-1", "mem-uuid-2"],
    "user_id": "user-123"
}
```

**Hidden Gem**: Semantic deduplication with 0.95 threshold — if an entity with similarity ≥ 0.95 exists, it's considered a match and updated rather than creating a duplicate.

### 3.6 Hybrid Search (BM25 + Vector)

Mem0 supports hybrid search when the vector store implements `keyword_search()`:

```python
# BM25 scoring with entity boost
score = (alpha * vector_score) + ((1 - alpha) * bm25_score) + (entity_boost * entity_overlap)
```

Stores supporting keyword search: Qdrant, Elasticsearch, PGVector, OpenSearch.

---

## 4. Observability Pipeline (Langfuse)

### 4.1 Ingestion Architecture

Langfuse uses a **queue-based ingestion pipeline** with ClickHouse as the analytics store:

```
SDK/API → Redis Queue → Worker → IngestionService → ClickHouse
                                              ↓
                                    Postgres (prompts, models, config)
```

**Key Abstractions** (from `worker/src/services/IngestionService/index.ts`):

| Component | Role |
|---|---|
| `IngestionService` | Merges events, enriches with model/pricing data, writes to ClickHouse |
| `ClickhouseWriter` | Batched async writer with queue-based buffering |
| `ClickhouseReadSkipCache` | Cache to avoid redundant reads during merge |
| `PromptService` | Resolves prompt name+version to prompt content |

### 4.2 Event Processing Pipeline

```typescript
class IngestionService {
    async mergeAndWrite(eventType, projectId, entityId, createdAtTimestamp, events) {
        switch (eventType) {
            case "trace":      return this.processTraceEventList(...)
            case "observation": return this.processObservationEventList(...)
            case "score":      return this.processScoreEventList(...)
            case "dataset_run_item": return this.processDatasetRunItemEventList(...)
        }
    }
}
```

**Hidden Gem — Immutable Entity Keys**: Each entity type has a set of keys that cannot be overwritten on update (e.g., `id`, `project_id`, `timestamp`). This prevents accidental data corruption.

### 4.3 Cost & Token Tracking

Langfuse enriches observations with:
- **Token counting**: Async tokenization fallback when provider doesn't report usage
- **Model matching**: Maps `provided_model_name` → internal model with pricing tiers
- **Cost calculation**: Per-token costs with tier-based pricing

### 4.4 Materialized Views Pattern

Langfuse uses ClickHouse materialized views for query optimization:
- `events_full` → raw event storage (write target)
- `events_core` → materialized view (auto-populated from events_full)
- Separate tables for traces, scores, observations with deduplication

---

## 5. Evaluation Metrics (Ragas)

### 5.1 Metric Architecture

Ragas metrics use a **mixin-based hierarchy** (from `ragas/metrics/base.py`):

```python
@dataclass
class Metric(ABC):
    name: str
    _required_columns: Dict[MetricType, Set[str]]
    
    @abstractmethod
    def init(self, run_config: RunConfig): ...

class MetricWithLLM(Metric, PromptMixin):
    llm: Optional[BaseRagasLLM]
    output_type: Optional[MetricOutputType]

class MetricWithEmbeddings(Metric):
    embeddings: Optional[BaseRagasEmbeddings]

class SingleTurnMetric(Metric):
    def single_turn_score(self, sample) -> float
    async def single_turn_ascore(self, sample) -> float

class MultiTurnMetric(Metric):
    def multi_turn_score(self, sample) -> float
```

**Metric Types**: `SINGLE_TURN`, `MULTI_TURN`
**Output Types**: `BINARY`, `DISCRETE`, `CONTINUOUS`, `RANKING`

### 5.2 Faithfulness Metric (NLI-based)

The Faithfulness metric (from `ragas/metrics/_faithfulness.py`) uses a two-step LLM pipeline:

```
Question + Answer → [StatementGenerator] → List[Statement]
Context + Statements → [NLI Verdict] → List[Verdict(0/1)]
Score = faithful_statements / total_statements
```

**Prompt Pattern** — Pydantic-typed prompts with examples:

```python
class StatementGeneratorPrompt(PydanticPrompt[StatementGeneratorInput, StatementGeneratorOutput]):
    instruction = "Break down each sentence into one or more fully understandable statements..."
    input_model = StatementGeneratorInput
    output_model = StatementGeneratorOutput
    examples = [(input_example, output_example), ...]
```

**Hidden Gem — HHEM Variant**: `FaithfulnesswithHHEM` replaces the LLM-based NLI with a local HuggingFace model (`vectara/hallucination_evaluation_model`) for offline evaluation.

### 5.3 Context Precision Metrics

Three variants (from `ragas/metrics/_context_precision.py`):

| Metric | Input | Method |
|---|---|---|
| `LLMContextPrecisionWithReference` | question, contexts, reference | LLM judges each context's usefulness |
| `LLMContextPrecisionWithoutReference` | question, contexts, response | LLM judges with response instead of reference |
| `NonLLMContextPrecisionWithReference` | retrieved_contexts, reference_contexts | String similarity + threshold |
| `IDBasedContextPrecision` | retrieved_ids, reference_ids | Direct ID set intersection |

**Average Precision Formula**:
```python
def _calculate_average_precision(self, verifications):
    cumsum = 0
    numerator = 0.0
    for i, ver in enumerate(verifications):
        v = 1 if ver.verdict else 0
        cumsum += v
        if v:
            numerator += cumsum / (i + 1)
    return numerator / (cumsum + 1e-10)
```

### 5.4 Metric Training (Prompt Optimization)

Metrics support two optimization strategies:

1. **Instruction Optimization**: Uses DSPy-style optimizers to refine prompt instructions
2. **Demonstration Optimization**: Builds few-shot examples from annotated data using semantic similarity

```python
metric.train(
    path="training_data.json",
    demonstration_config=DemonstrationConfig(embedding=..., top_k=5),
    instruction_config=InstructionConfig(optimizer=..., llm=...),
)
```

---

## 6. Token Efficiency Patterns

### 6.1 Context Window Management

**Chunking Strategies** (from LlamaIndex node parsers):
- `SentenceSplitter`: Splits on sentence boundaries with overlap
- `TokenTextSplitter`: Splits on token count boundaries
- `CodeSplitter`: AST-aware splitting for code files
- `SemanticSplitter`: Uses embeddings to find natural break points

### 6.2 Context Compression

**Strategies ranked by effectiveness**:
1. **Reranking**: Use cross-encoder to reorder retrieved chunks (Mem0's `RerankerFactory`)
2. **Filtering**: Remove low-relevance chunks below threshold
3. **Extraction**: Extract only relevant sentences from chunks
4. **Summary**: LLM-summarize retrieved context before synthesis

### 6.3 Lazy Evaluation

From LlamaIndex's `BaseRetriever`:
```python
# Nodes are lazily evaluated - expensive operations deferred until needed
@dispatcher.span
def retrieve(self, str_or_query_bundle):
    nodes = self._retrieve(query_bundle)
    nodes = self._handle_recursive_retrieval(query_bundle, nodes)  # Lazy resolution
    return nodes
```

---

## 7. Implementation Patterns

### 7.1 Pydantic Prompt Pattern (Ragas)

```python
class MyPrompt(PydanticPrompt[InputModel, OutputModel]):
    instruction = "Your instruction here"
    input_model = InputModel
    output_model = OutputModel
    examples = [
        (InputModel(field1="...", field2="..."), 
         OutputModel(result="...")),
    ]

# Usage
output = await prompt.generate(data=input_data, llm=self.llm, callbacks=callbacks)
```

### 7.2 Factory + Lazy Loading Pattern (Mem0)

```python
class VectorStoreFactory:
    provider_to_class = {
        "qdrant": "mem0.vector_stores.qdrant.Qdrant",
        "chroma": "mem0.vector_stores.chroma.ChromaDB",
        # ... 24 providers
    }
    
    @classmethod
    def create(cls, provider_name, config):
        class_type = cls.provider_to_class.get(provider_name)
        if not isinstance(config, dict):
            config = config.model_dump()
        vector_store_instance = load_class(class_type)  # Lazy import
        return vector_store_instance(**config)
```

### 7.3 Callback/Dispatcher Pattern (LlamaIndex)

```python
# Dual callback system: legacy CallbackManager + modern Dispatcher
class BaseRetriever(PromptMixin, DispatcherSpanMixin):
    @dispatcher.span  # OpenTelemetry-compatible span
    def retrieve(self, str_or_query_bundle):
        dispatcher.event(RetrievalStartEvent(...))
        with self.callback_manager.as_trace("query"):
            with self.callback_manager.event(CBEventType.RETRIEVE, payload={...}):
                nodes = self._retrieve(query_bundle)
        dispatcher.event(RetrievalEndEvent(...))
        return nodes
```

### 7.4 Event-Driven Ingestion (Langfuse)

```typescript
// Immutable keys prevent accidental overwrites
const immutableEntityKeys = {
    [TableName.Traces]: ["id", "project_id", "timestamp", "created_at", "environment"],
    [TableName.Scores]: ["id", "project_id", "timestamp", "trace_id", "created_at", "environment"],
    [TableName.Observations]: ["id", "project_id", "trace_id", "start_time", "created_at", "environment"],
};
```

---

## 8. Anti-Patterns to Avoid

1. **No score normalization**: Vector stores returning raw distances instead of similarity scores — breaks threshold-based filtering
2. **Eager entity import**: Importing all vector store providers at startup instead of lazy loading
3. **Missing deduplication**: Not checking for existing memories before inserting — leads to vector store bloat
4. **Synchronous embedding in async pipeline**: Blocking on embedding calls in async code
5. **No version context in distributed queries**: Querying vector and metadata segments without `RequestVersionContext` — can return inconsistent results
6. **Hard-coded prompts**: Not using the PydanticPrompt pattern — prevents prompt optimization and few-shot learning
7. **Ignoring the `:optional` column suffix**: Ragas metrics support optional columns that shouldn't be required but can improve scoring


## Verification Checklist

- [ ] Chunk size tested empirically (not just default)
- [ ] Embedding model validated on domain-specific queries
- [ ] Retrieval precision/recall measured on test set
- [ ] Context window budget enforced (token counting)
- [ ] Hybrid search (dense + sparse) considered
- [ ] Re-ranking applied after retrieval
- [ ] Hallucination rate measured
- [ ] Memory persistence tested across sessions
- [ ] Evaluation metrics computed (faithfulness, relevancy, context precision)
- [ ] Cost per query tracked


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

- `mcp-tools-model-routing`
- `ai-agent-orchestration`

