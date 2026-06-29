# AI Agent Orchestration — Deep Reference Skill

> Synthesized from source analysis of **LangGraph**, **AutoGen**, **CrewAI**, **OpenHands**, and **Aider**.

---

## Overview

AI Agent Orchestration covers autonomous agent loops, multi-agent fleet coordination, state machines, role-based agents, checkpointing, human-in-the-loop patterns, event-driven architectures, sandboxed execution, and self-improvement engines. Extracted from deep analysis of LangGraph, AutoGen, CrewAI, OpenHands, and aider.

**When to use**: Building, debugging, or optimizing systems in this domain.

## 1. Top 10 Orchestration Patterns

### Pattern 1: State Machine Graphs (LangGraph)

**What:** Nodes communicate via shared typed state. Edges define transitions. Reducers aggregate concurrent writes.

**Key Abstractions:**
```python
# StateGraph — builder for stateful graph workflows
class StateGraph(Generic[StateT, ContextT, InputT, OutputT]):
    def __init__(self, state_schema: type[StateT], context_schema: type[ContextT] | None = None, *,
                 input_schema: type[InputT] | None = None, output_schema: type[OutputT] | None = None): ...
    def add_node(self, node: StateNode[NodeInputT, ContextT], *, defer: bool = False,
                 metadata: dict[str, Any] | None = None, input_schema: type[NodeInputT] | None = None,
                 retry_policy: RetryPolicy | Sequence[RetryPolicy] | None = None,
                 cache_policy: CachePolicy | None = None,
                 error_handler: StateNode[Any, ContextT] | None = None,
                 destinations: dict[str, str] | tuple[str, ...] | None = None,
                 timeout: float | timedelta | TimeoutPolicy | None = None) -> Self: ...
    def add_edge(self, source: str | list[str], target: str) -> Self: ...
    def add_conditional_edges(self, source: str, path: Callable | Runnable,
                              path_map: dict | list | None = None) -> Self: ...
    def compile(self, checkpointer: Checkpointer = None, *, interrupt_before: list[str] | All = None,
                interrupt_after: list[str] | All = None) -> CompiledStateGraph: ...

# Node signature — state in, partial state out
StateNode: TypeAlias = (
    _Node[NodeInputT] | _NodeWithConfig[NodeInputT] | _NodeWithWriter[NodeInputT] |
    _NodeWithStore[NodeInputT] | _NodeWithRuntime[NodeInputT, ContextT] | Runnable[NodeInputT, Any]
)

# StateNodeSpec — per-node metadata
@dataclass(slots=True)
class StateNodeSpec(Generic[NodeInputT, ContextT]):
    runnable: StateNode[NodeInputT, ContextT]
    metadata: dict[str, Any] | None
    input_schema: type[NodeInputT]
    retry_policy: RetryPolicy | Sequence[RetryPolicy] | None
    cache_policy: CachePolicy | None
    is_error_handler: bool = False
    error_handler_node: str | None = None
    ends: tuple[str, ...] | dict[str, str] | None = EMPTY_SEQ
    defer: bool = False
    timeout: TimeoutPolicy | None = None

# BranchSpec — conditional routing
class BranchSpec(NamedTuple):
    path: Runnable[Any, Hashable | list[Hashable]]
    ends: dict[Hashable, str] | None
    input_schema: type[Any] | None = None
```

**Channel System:**
```python
# Channels are the communication primitives between nodes
class BaseChannel(ABC, Generic[ValueT, UpdateT]):
    @abstractmethod
    def checkpoint(self) -> Checkpoint | None: ...
    @abstractmethod
    def consume(self) -> ValueT: ...
    @abstractmethod
    def update(self, values: Sequence[UpdateT]) -> bool: ...

# Built-in channel types:
# - LastValue: stores single latest value (default for most state keys)
# - BinaryOperatorAggregate: applies reducer function (a, b) -> a
# - EphemeralValue: cleared after each step
# - NamedBarrierValue: waits for N writes before releasing
# - DeltaChannel: incremental updates with snapshot frequency
```

**Hidden Gems:**
- `defer=True` on nodes delays execution until the graph is about to end (useful for summarization)
- `Command` objects let nodes dynamically route: `return Command(goto="node_name", update={"key": value})`
- `Send` enables dynamic fan-out: `return [Send("worker", {"input": item}) for item in items]`
- `set_node_defaults()` applies retry/cache/error policies globally
- `TimeoutPolicy` supports per-node timeouts via `timeout=float|timedelta|TimeoutPolicy`
- Context schema (`context_schema`) provides immutable runtime context (user_id, db_conn) separate from mutable state
- `interrupt()` pauses execution and surfaces values to the caller for human-in-the-loop
- `Durability` modes: `"sync"` (persist before next step), `"async"` (persist while next step runs), `"exit"` (persist only on exit)

**Anti-Patterns:**
- Don't mutate state directly — always return partial state dicts
- Don't use `config_schema` (deprecated) — use `context_schema`
- Don't import `Send`/`Interrupt` from `langgraph.constants` (deprecated) — use `langgraph.types`
- Avoid deeply nested subgraphs without checkpointing — state is lost on failure

---

### Pattern 2: Role-Based Agent Teams (CrewAI)

**What:** Agents have roles, goals, and backstories. Tasks have descriptions and expected outputs. Crews orchestrate execution.

**Key Abstractions:**
```python
# Agent — role-based LLM-powered entity
class Agent(BaseModel):
    role: str                                    # e.g., "Senior Data Analyst"
    goal: str                                    # e.g., "Uncover hidden insights"
    backstory: str                               # e.g., "With 10 years of experience..."
    llm: BaseLLM | str | None                   # LLM reference (litellm model string or BaseLLM)
    tools: list[BaseTool] | None                # Available tools
    max_iter: int = 20                           # Max iterations per task
    max_rpm: int | None = None                   # Rate limiting
    max_execution_time: int | None = None        # Timeout in seconds
    memory: bool = True                          # Enable memory
    knowledge: Knowledge | list[BaseKnowledgeSource] | None = None
    mcp_servers: list[MCServerConfig] | None = None  # MCP tool sources
    skills: list[Skill] | None = None            # Skill activation
    checkpoint: CheckpointConfig | bool | None = None

# Task — unit of work with expected output
class Task(BaseModel):
    description: str                             # What to do
    expected_output: str                         # What the result should look like
    agent: BaseAgent | None = None               # Assigned agent
    tools: list[BaseTool] | None = None          # Task-specific tools
    context: list[Task] | None = None            # Dependent tasks
    output_json: type[BaseModel] | None = None   # Structured output schema
    output_pydantic: type[BaseModel] | None = None
    output_file: str | None = None               # Write output to file
    output_format: OutputFormat | None = None
    guardrails: GuardrailsType | None = None     # LLM guardrails
    async_execution: bool = False
    callback: SerializableCallable | None = None

# Crew — orchestrator of agents and tasks
class Crew(FlowTrackable, BaseModel):
    agents: list[BaseAgent]
    tasks: list[Task]
    process: Process = Process.sequential         # sequential | hierarchical
    manager_llm: BaseLLM | str | None = None      # For hierarchical process
    manager_agent: Agent | None = None             # Custom manager
    verbose: bool | int = False
    memory: bool = False
    cache: bool = True
    max_rpm: int | None = None
    planning: bool = False                         # Auto-generate plan
    planning_llm: BaseLLM | str | None = None
    function_calling_llm: BaseLLM | str | None = None
    share_crew: bool = False
    output_log_file: str | None = None
    embedder: EmbedderConfig | None = None
    knowledge: Knowledge | list[BaseKnowledgeSource] | None = None
    
    def kickoff(self, inputs: dict | None = None) -> CrewOutput: ...
    async def kickoff_async(self, inputs: dict | None = None) -> CrewOutput: ...
    def train(self, n_iterations: int, filename: str, inputs: dict | None = None) -> None: ...
    def test(self, n_iterations: int, openai_model_name: str, inputs: dict | None = None) -> None: ...

# Process types
class Process(str, Enum):
    sequential = "sequential"    # Tasks run one after another
    hierarchical = "hierarchical"  # Manager agent delegates to workers
```

**Hidden Gems:**
- `ConditionalTask` — tasks that execute only when a condition is met
- `CrewPlanner` — auto-generates execution plans when `planning=True`
- LLM guardrails (`hallucination_guardrail`, `llm_guardrail`) validate task outputs
- MCP tool integration via `MCServerConfig` — agents can use MCP tools natively
- Skills system — `discover_skills()` and `activate_skill()` for dynamic capability loading
- `CrewEvaluator` — self-improvement via automated crew evaluation
- `CheckpointConfig` — resume crew execution from failure points
- Knowledge sources with RAG embeddings for agent context
- `Fingerprint` security for crew/agent identity verification

**Anti-Patterns:**
- Don't set `max_iter` too high — agents can loop indefinitely on ambiguous tasks
- Don't use `share_crew=True` in production — sends data to CrewAI cloud
- Avoid `hierarchical` process without `manager_llm` — uses default model
- Don't mix `output_json` and `output_pydantic` — pick one

---

### Pattern 3: Message-Passing Agent Runtime (AutoGen)

**What:** Agents communicate via typed messages through a runtime. Pub/sub via topics. Agent types registered with subscriptions.

**Key Abstractions:**
```python
# BaseAgent — abstract agent with message handling
class BaseAgent(ABC, Agent):
    def __init__(self, description: str) -> None: ...
    @abstractmethod
    async def on_message_impl(self, message: Any, ctx: MessageContext) -> Any: ...
    async def send_message(self, message: Any, recipient: AgentId, *,
                           cancellation_token: CancellationToken | None = None) -> Any: ...
    async def publish_message(self, message: Any, topic_id: TopicId, *,
                              cancellation_token: CancellationToken | None = None) -> None: ...
    async def save_state(self) -> Mapping[str, Any]: ...
    async def load_state(self, state: Mapping[str, Any]) -> None: ...

# AgentId — unique agent identifier (type + key)
class AgentId:
    type: str   # Agent type name
    key: str    # Instance key

# TopicId — pub/sub topic identifier
class TopicId:
    type: str   # Topic type
    source: str # Topic source

# Decorators for agent registration
@handles(type: Type[Any], serializer: MessageSerializer | None = None)
@subscription_factory(subscription: UnboundSubscription)

# AgentRuntime — manages agent lifecycle and message routing
class AgentRuntime(ABC):
    async def send_message(self, message: Any, sender: AgentId, recipient: AgentId, ...) -> Any: ...
    async def publish_message(self, message: Any, topic_id: TopicId, sender: AgentId, ...) -> None: ...
    async def register_agent_instance(self, agent_instance: BaseAgent, agent_id: AgentId) -> AgentId: ...
    async def add_subscription(self, subscription: Subscription) -> None: ...

# Subscription types
class TypeSubscription:      # Routes messages by topic type
class TypePrefixSubscription: # Routes by topic type prefix
class DefaultSubscription:    # Routes to default topic

# High-level agents (autogen-agentchat)
class AssistantAgent:        # LLM-powered agent with tools
class CodeExecutorAgent:     # Executes code in sandbox
class UserProxyAgent:        # Human-in-the-loop proxy
class SocietyOfMindAgent:    # Nested multi-agent team
class MessageFilterAgent:    # Filters messages by criteria
```

**Hidden Gems:**
- `SocietyOfMindAgent` — wraps a multi-agent team as a single agent (composability)
- `@handles` decorator — auto-registers message type handlers with serializers
- `register_instance()` — runtime instance registration with direct message subscriptions
- `CancellationToken` — cooperative cancellation across agent chains
- `AgentInstantiationContext` — factory pattern for agent creation within runtime
- Intervention handlers — intercept/modify messages before delivery
- Cross-language support via gRPC (Python ↔ .NET agents)

**Anti-Patterns:**
- Don't call `on_message_impl` directly — use `on_message` (adds middleware hooks)
- Don't share `internal_extra_handles_types` between subclasses — auto-reset per subclass
- Avoid tight coupling between agents — use topics for loose coupling

---

### Pattern 4: Event-Driven Agent Runtime (OpenHands)

**What:** Agents operate on an event stream. Actions produce observations. Sandboxed execution environment.

**Key Abstractions:**
```python
# EventService — event storage and retrieval
class EventService(ABC):
    async def get_event(self, conversation_id: UUID, event_id: UUID) -> Event | None: ...
    async def search_events(self, conversation_id: UUID, kind__eq: EventKind | None = None,
                            timestamp__gte: datetime | None = None, ...) -> EventPage: ...
    async def save_event(self, conversation_id: UUID, event: Event): ...
    async def count_events(self, conversation_id: UUID, ...) -> int: ...
    async def iter_events_for_export(self, conversation_id: UUID) -> AsyncGenerator[Event, None]: ...

# Event model — base for all events
class Event(DiscriminatedUnionMixin):
    # Actions: CmdRunAction, FileReadAction, FileWriteAction, BrowseURLAction, etc.
    # Observations: CmdOutputObservation, FileReadObservation, ErrorObservation, etc.

# App architecture:
# - app_server/ — FastAPI application with REST endpoints
# - event/ — EventService implementations (filesystem, AWS, Google Cloud)
# - sandbox/ — Sandboxed execution environments
# - integrations/ — GitHub, GitLab, Bitbucket, Jira integrations
```

**Hidden Gems:**
- Multiple event storage backends (filesystem, AWS S3, Google Cloud Storage)
- Event callback system with webhook notifications
- MCP integration in `app_server/mcp/` — OpenHands as MCP server
- Conversation-level isolation with git-based state management
- V1 router architecture with middleware pipeline

**Anti-Patterns:**
- Don't bypass EventService for event storage — breaks event sourcing guarantees
- Don't assume events are ordered by insertion — use timestamp ordering

---

### Pattern 5: Checkpointing & State Persistence (LangGraph)

**What:** Crash-safe state snapshots with version tracking, fork support, and multiple backends.

**Key Abstractions:**
```python
# Checkpoint — state snapshot at a point in time
class Checkpoint(TypedDict):
    v: int                          # Format version (currently 1)
    id: str                         # Unique, monotonically increasing ID
    ts: str                         # ISO 8601 timestamp
    channel_values: dict[str, Any]  # Deserialized channel values
    channel_versions: ChannelVersions  # Per-channel version tracking
    versions_seen: dict[str, ChannelVersions]  # Node -> channel version map
    updated_channels: list[str] | None  # Channels changed in this checkpoint

# CheckpointMetadata — metadata about checkpoint creation
class CheckpointMetadata(TypedDict, total=False):
    source: Literal["input", "loop", "update", "fork"]
    step: int                       # -1 for input, 0 for first loop, etc.
    parents: dict[str, str]         # Namespace -> checkpoint ID mapping
    run_id: str

# CheckpointTuple — checkpoint with its context
class CheckpointTuple(NamedTuple):
    config: RunnableConfig
    checkpoint: Checkpoint
    metadata: CheckpointMetadata
    parent_config: RunnableConfig | None = None
    pending_writes: list[PendingWrite] | None = None

# BaseCheckpointSaver — abstract checkpoint storage
class BaseCheckpointSaver(ABC, Generic[SerT]):
    async def aget_tuple(self, config: RunnableConfig) -> CheckpointTuple | None: ...
    async def aput(self, config: RunnableConfig, checkpoint: Checkpoint,
                   metadata: CheckpointMetadata, new_versions: ChannelVersions) -> RunnableConfig: ...
    async def aput_writes(self, config: RunnableConfig, writes: list[PendingWrite],
                          task_id: str) -> None: ...
    async def adelete_thread(self, thread_id: str) -> None: ...

# Built-in implementations:
# - InMemorySaver — in-memory (development/testing)
# - AsyncPostgresSaver — PostgreSQL (production)
# - AsyncSqliteSaver — SQLite (lightweight production)
```

**Hidden Gems:**
- `Durability` modes: `"sync"`, `"async"`, `"exit"` control when checkpoints are written
- Fork support — create branches from any checkpoint
- `DeltaChannel` with snapshot frequency for incremental state tracking
- Encrypted serializer for sensitive state data
- `CachePolicy` with `default_cache_key` using xxhash for fast hashing
- `pending_writes` — writes that haven't been committed yet (crash recovery)

---

### Pattern 6: Human-in-the-Loop (LangGraph + AutoGen)

**What:** Pause execution, surface state to humans, resume with input.

**Key Abstractions:**
```python
# LangGraph interrupt
def interrupt(value: Any) -> Any:
    """Pause execution and surface value to caller. Returns resume value."""
    ...

# Interrupt in state
class Interrupt(TypedDict):
    value: Any          # Value surfaced to human
    resumable: bool     # Whether this can be resumed
    ns: list[str]       # Namespace path

# AutoGen UserProxyAgent
class UserProxyAgent:
    """Agent that proxies human input in multi-agent conversations."""
    # Supports: text input, code execution approval, tool call approval

# State snapshot for inspection
class StateSnapshot:
    values: Any         # Current state
    next: tuple[str, ...]  # Next nodes to execute
    config: RunnableConfig
    metadata: CheckpointMetadata
    tasks: tuple[PregelTask, ...]
```

**Pattern:** Call `interrupt()` in a node → execution pauses → caller inspects `StateSnapshot` → calls `graph.invoke(Command(resume=value))` to continue.

---

### Pattern 7: Sandboxed Code Execution (OpenHands + AutoGen)

**What:** Execute untrusted code in isolated environments with resource limits.

**AutoGen CodeExecutorAgent:**
```python
class CodeExecutorAgent:
    """Executes code blocks from messages in a sandboxed environment."""
    # Supports: Docker, local, Azure Container Apps
    # Extracts code blocks from messages → executes → returns output
```

**OpenHands Sandboxing:**
- Docker-based sandboxing with resource limits
- Local runtime with tmux session isolation
- Playwright browser sandboxing for web interactions
- File system isolation per conversation

---

### Pattern 8: Repository Maps (Aider)

**What:** Build a compact representation of a codebase using tree-sitter AST parsing.

**Key Abstractions:**
```python
class RepoMap:
    def __init__(self, map_tokens=1024, root=None, main_model=None, io=None,
                 repo_content_prefix=None, max_context_window=None,
                 map_mul_no_files=8, refresh="auto"): ...
    
    def get_repo_map(self, chat_files: list[str], other_files: list[str],
                     mentioned_fnames: set[str] | None = None,
                     mentioned_idents: set[str] | None = None) -> str | None: ...
    
    # Uses tree-sitter to extract:
    # - Function/class definitions and signatures
    # - Import statements
    # - Module-level variables
    # - Tags cache (.aider.tags.cache.v4/) for incremental updates

# Tag extraction
Tag = namedtuple("Tag", "rel_fname fname line name kind".split())

# Cache versioning for incremental updates
CACHE_VERSION = 4  # Bumps on tree-sitter pack changes
```

**Hidden Gems:**
- Token-aware truncation — respects `map_tokens` budget
- `map_mul_no_files=8` — when no files are in chat, uses 8x token budget
- Importance scoring via `filter_important_files()` for large codebases
- Disk cache with SQLite for tag persistence across sessions
- Pygments lexer fallback when tree-sitter isn't available

---

### Pattern 9: Tool Registries & Function Calling (CrewAI + Aider)

**What:** Dynamic tool registration, validation, and execution with LLM function calling.

**CrewAI Tool System:**
```python
class BaseTool(BaseModel):
    name: str
    description: str
    args_schema: type[BaseModel] | None = None
    fn: Callable | None = None
    
    def run(self, **kwargs) -> Any: ...

class StructuredTool(BaseTool):
    """Tool with structured input/output via Pydantic models."""

# MCP tool integration
class MCPServerConfig(BaseModel):
    """Configuration for MCP server connection."""
    # Enables agents to use tools from MCP servers

# Tool usage with guardrails
class ToolUsage:
    def __init__(self, tools: list[BaseTool], llm: BaseLLM, ...) -> None: ...
    def use(self, tool_name: str, tool_input: dict) -> str: ...
```

**Aider Edit Formats:**
```python
class Coder:
    edit_format: str  # "diff", "whole", "udiff", "patch", "architect", etc.
    
    @classmethod
    def create(cls, main_model=None, edit_format=None, io=None, from_coder=None, **kwargs):
        """Factory method — selects coder implementation by edit_format."""
        # Iterates coders.__all__ to find matching edit_format
        # Supports: EditBlockCoder, WholeFileCoder, UDiffCoder, PatchCoder, ArchitectCoder
    
    def run(self, with_message: str) -> None:
        """Main loop: build messages → call LLM → apply edits → lint → commit."""
```

---

### Pattern 10: Self-Improvement & Training (CrewAI + Aider)

**What:** Agents learn from execution history to improve future performance.

**CrewAI Training:**
```python
class Crew:
    def train(self, n_iterations: int, filename: str, inputs: dict | None = None) -> None:
        """Train agents by running tasks and recording successful patterns."""
        # Uses CrewTrainingHandler to persist training data
        # TaskEvaluator scores outputs
        # CrewEvaluator aggregates scores across iterations
    
    def test(self, n_iterations: int, openai_model_name: str, inputs: dict | None = None) -> None:
        """Test crew with automated evaluation."""
```

**Aider Self-Improvement:**
- `ChatSummary` — summarizes long conversations to stay within context
- `max_reflections=3` — limits self-correction attempts
- Auto-lint and auto-test after each edit
- `Linter` integration catches syntax errors before committing

---

## 2. Pattern Matrix

| Pattern | LangGraph | AutoGen | CrewAI | OpenHands | Aider |
|---------|-----------|---------|--------|-----------|-------|
| State Machine | ✅ Core | ✅ Runtime | ❌ | ✅ Events | ❌ |
| Role-Based Agents | ❌ | ✅ | ✅ Core | ❌ | ✅ Architect/Editor |
| Graph Orchestration | ✅ Core | ✅ Topics | ❌ | ❌ | ❌ |
| Checkpointing | ✅ Core | ✅ State | ✅ | ✅ Events | ❌ |
| Human-in-the-Loop | ✅ interrupt() | ✅ UserProxy | ❌ | ✅ | ✅ IO confirm |
| Sandboxed Execution | ❌ | ✅ CodeExec | ❌ | ✅ Core | ❌ |
| Repo Maps | ❌ | ❌ | ❌ | ❌ | ✅ Core |
| Tool Registries | ✅ Tools | ✅ | ✅ Core | ✅ | ✅ |
| Self-Improvement | ❌ | ❌ | ✅ Train | ❌ | ✅ Reflect |
| Event-Driven | ❌ | ✅ Pub/Sub | ✅ Events | ✅ Core | ❌ |

---

## 3. Library Selection Guide

### Choose LangGraph when:
- You need fine-grained control over agent state and transitions
- Checkpointing and crash recovery are critical
- You want conditional routing and dynamic fan-out
- You need human-in-the-loop workflows
- You're building complex multi-step pipelines

### Choose AutoGen when:
- You need multi-agent conversations with message passing
- Code execution in sandboxed environments is required
- Cross-language agent communication (Python ↔ .NET)
- Pub/sub topic-based routing
- You want composable agent teams (SocietyOfMind)

### Choose CrewAI when:
- You want a high-level, opinionated framework
- Role-based agent teams with natural language roles
- Built-in training and self-improvement
- MCP tool integration out of the box
- Sequential or hierarchical process flows

### Choose OpenHands when:
- You're building a coding assistant / autonomous developer
- Sandboxed code execution is the primary use case
- Event sourcing and conversation management are needed
- Git integration and file management are core requirements

### Choose Aider when:
- You're building a code editing assistant
- Repository understanding via tree-sitter is valuable
- Multiple edit formats (diff, whole, patch) are needed
- Git-native workflow (auto-commit, blame, history)
- Lightweight, single-agent focused

---

## 4. Implementation Internals

### LangGraph Execution Engine (Pregel)

The core execution engine is called **Pregel** (inspired by Google's Pregel graph processing system):

```python
# Pregel execution loop (simplified):
# 1. Read current checkpoint
# 2. Determine which nodes to execute (based on channel versions)
# 3. Execute nodes in parallel (respecting dependencies)
# 4. Apply node outputs to channels via ChannelWrite
# 5. Apply channel reducers (BinaryOperatorAggregate, etc.)
# 6. Write checkpoint
# 7. Repeat until no more nodes to execute

# Key files:
# - pregel/__init__.py — Pregel class (main execution engine)
# - pregel/_loop.py — AsyncPregelLoop (core execution loop)
# - pregel/_algo.py — Task scheduling algorithm
# - pregel/_runner.py — Task execution with retry
# - pregel/_executor.py — Thread pool executor
# - pregel/_write.py — ChannelWrite (output application)
# - pregel/_read.py — ChannelRead (input reading)
```

### AutoGen Runtime Architecture

```python
# SingleThreadedAgentRuntime — default runtime
class SingleThreadedAgentRuntime(AgentRuntime):
    # Message queue with asyncio
    # Agent factory pattern for lazy instantiation
    # Subscription-based routing (TypeSubscription, TypePrefixSubscription)
    # Intervention handlers for message interception
    # State persistence via save_state/load_state
```

### CrewAI Execution Flow

```python
# Crew.kickoff() flow:
# 1. prepare_kickoff() — validate agents, tasks, setup memory/cache
# 2. If planning=True → CrewPlanner generates execution plan
# 3. For each task (sequential or hierarchical):
#    a. prepare_task_execution() — resolve agent, tools, context
#    b. agent.execute_task(task) → CrewAgentExecutor
#    c. Process guardrails (hallucination check, LLM validation)
#    d. Store task output, update memory
# 4. Return CrewOutput (raw + pydantic + json + token_usage)
```

### Aider Edit Loop

```python
# Coder.run() flow:
# 1. Build chat messages (system prompt + repo map + file contents + user message)
# 2. Call LLM with streaming
# 3. Parse response for edit blocks (format-specific)
# 4. Apply edits to files
# 5. Auto-lint changed files
# 6. Auto-test if configured
# 7. Git commit with descriptive message
# 8. Update chat history
```

---

## 5. Code Examples

### Minimal LangGraph Agent
```python
from typing_extensions import TypedDict, Annotated
from langgraph.graph import StateGraph, START, END
from langgraph.checkpoint.memory import InMemorySaver

class State(TypedDict):
    messages: Annotated[list, lambda a, b: a + b]
    current_step: str

def researcher(state: State) -> dict:
    # Research logic here
    return {"messages": [("assistant", "Research complete")], "current_step": "writing"}

def writer(state: State) -> dict:
    # Writing logic here
    return {"messages": [("assistant", "Draft complete")], "current_step": "done"}

def should_continue(state: State) -> str:
    return "writer" if state["current_step"] == "writing" else END

graph = StateGraph(State)
graph.add_node("researcher", researcher)
graph.add_node("writer", writer)
graph.add_edge(START, "researcher")
graph.add_conditional_edges("researcher", should_continue)
graph.add_edge("writer", END)

app = graph.compile(checkpointer=InMemorySaver())
result = app.invoke({"messages": [], "current_step": ""}, config={"configurable": {"thread_id": "1"}})
```

### Minimal CrewAI Team
```python
from crewai import Agent, Task, Crew, Process

researcher = Agent(
    role="Research Analyst",
    goal="Find comprehensive information about the topic",
    backstory="Expert researcher with 10 years of experience",
    verbose=True
)

writer = Agent(
    role="Content Writer",
    goal="Write engaging, accurate content",
    backstory="Professional writer with expertise in technical content",
    verbose=True
)

research_task = Task(
    description="Research the latest developments in AI agent orchestration",
    expected_output="A comprehensive report with key findings",
    agent=researcher
)

write_task = Task(
    description="Write a blog post based on the research",
    expected_output="A 1000-word blog post",
    agent=writer,
    context=[research_task]
)

crew = Crew(
    agents=[researcher, writer],
    tasks=[research_task, write_task],
    process=Process.sequential,
    verbose=True
)

result = crew.kickoff()
```

### Minimal AutoGen Multi-Agent
```python
from autogen_core import BaseAgent, AgentId, MessageContext
from autogen_agentchat.agents import AssistantAgent, CodeExecutorAgent

class MyAgent(BaseAgent):
    async def on_message_impl(self, message: Any, ctx: MessageContext) -> Any:
        # Process message and return response
        return {"response": "Processed"}

# Registration with runtime
agent_id = AgentId(type="my_agent", key="instance_1")
await runtime.register_agent_instance(MyAgent("My agent"), agent_id)
```

---

## 6. Verification Checklist

- [ ] State schema uses `TypedDict` or `Pydantic BaseModel` with proper type annotations
- [ ] Reducers defined for list/aggregate fields via `Annotated[type, reducer_fn]`
- [ ] Checkpointing enabled for production workloads (`checkpointer=PostgresSaver()`)
- [ ] Retry policies set for LLM-calling nodes (`retry_policy=RetryPolicy(max_attempts=3)`)
- [ ] Timeout policies set for long-running nodes (`timeout=TimeoutPolicy(seconds=300)`)
- [ ] Error handlers registered for fault-tolerant graphs (`error_handler=fallback_node`)
- [ ] Human-in-the-loop points identified and `interrupt()` used appropriately
- [ ] Tool schemas validated (Pydantic `args_schema` or JSON Schema)
- [ ] Agent roles are specific and actionable (not vague like "helpful assistant")
- [ ] Task descriptions include clear expected output format
- [ ] Memory/cache configured appropriately (not in hot loops without TTL)
- [ ] Rate limiting set for external API calls (`max_rpm`)
- [ ] Streaming enabled for interactive applications (`stream_mode="messages"`)
- [ ] Subgraphs used for composability (not monolithic graphs)
- [ ] Context schema separates immutable runtime config from mutable state

---

## 7. Anti-Patterns Summary

| Anti-Pattern | Impact | Fix |
|--------------|--------|-----|
| Mutable state sharing | Race conditions, data corruption | Use reducers, return partial updates |
| No checkpointing | Lost work on crashes | Enable `InMemorySaver` or `PostgresSaver` |
| Infinite agent loops | Runaway costs, timeouts | Set `max_iter`, `max_execution_time` |
| Vague agent roles | Poor quality outputs | Specific roles with clear goals |
| No error handling | Silent failures | Register `error_handler` nodes |
| Skipping guardrails | Hallucinated outputs | Use `hallucination_guardrail` |
| Deep subgraph nesting | Debugging nightmares | Flatten, use named nodes |
| Ignoring token limits | Context overflow | Use `RepoMap`, `ChatSummary`, pruning |
| Synchronous in async | Event loop blocking | Use `ainvoke()`, `akickoff()` |
| Hardcoded model names | Vendor lock-in | Use abstraction layer (litellm, BaseLLM) |


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

- `machine-learning-pipelines`
- `reinforcement-learning-pipelines`

