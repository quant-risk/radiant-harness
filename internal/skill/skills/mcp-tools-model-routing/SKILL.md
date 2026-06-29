# MCP Tools & Model Routing — Deep Reference Skill

> Synthesized from source analysis of **MCP Python SDK**, **MCP Go SDK**, **LiteLLM**, **go-openai**, and **Continue**.

---

## Overview

MCP, Tools & Model Routing covers the Model Context Protocol (MCP) internals, tool registration patterns, function calling, model routing strategies, LLM client abstraction, streaming, retry logic, and cost tracking. Extracted from deep analysis of MCP Python/Go SDKs, LiteLLM, go-openai, and Continue.

**When to use**: Building, debugging, or optimizing systems in this domain.

## 1. MCP Protocol Internals

### Protocol Overview

The Model Context Protocol (MCP) is a JSON-RPC 2.0 based protocol for connecting LLM applications with external tools, resources, and prompts. It supports multiple transports (stdio, SSE, Streamable HTTP).

### Core Protocol Messages

```python
# Request types (from mcp_types)
InitializeRequest        # Client → Server: handshake with capabilities
CallToolRequest          # Client → Server: invoke a tool
ListToolsResult          # Server → Client: available tools
ListResourcesResult      # Server → Client: available resources
ReadResourceRequest      # Client → Server: read resource contents
GetPromptRequest         # Client → Server: get prompt template
ListPromptsResult        # Server → Client: available prompts
CompleteRequest          # Client → Server: autocomplete
SubscribeRequest         # Client → Server: resource change subscription

# Capabilities negotiation
class ServerCapabilities:
    tools: ToolsCapability | None
    resources: ResourcesCapability | None
    prompts: PromptsCapability | None
    logging: LoggingLevel | None
    sampling: SamplingCapability | None
    roots: RootsCapability | None

class ClientCapabilities:
    roots: RootsCapability | None
    sampling: SamplingCapability | None
    elicitation: dict | None
```

### Transport Layer

```python
# Stdio transport — process-based communication
class StdioServerParameters(BaseModel):
    command: str              # e.g., "python", "node"
    args: list[str] | None    # e.g., ["-m", "my_server"]
    env: dict[str, str] | None
    cwd: str | None

async def stdio_client(server: StdioServerParameters) -> AsyncIterator[Transport]: ...

# Streamable HTTP transport — HTTP-based with streaming
async def streamable_http_client(url: str, *, headers: dict | None = None) -> AsyncIterator[Transport]: ...

# SSE transport — Server-Sent Events (legacy)
class SseServerTransport:
    """Legacy SSE transport for backward compatibility."""

# In-memory transport — for testing
class InMemoryTransport:
    """Direct in-process communication via memory streams."""
```

---

## 2. MCP Python SDK Architecture

### Server Side (FastMCP / MCPServer)

```python
# MCPServer — high-level server interface (v2 API)
class MCPServer(Generic[LifespanResultT]):
    def __init__(self, name: str | None = None, title: str | None = None,
                 description: str | None = None, instructions: str | None = None,
                 version: str | None = None, *, tools: list[Tool] | None = None,
                 resources: list[Resource] | None = None,
                 extensions: Sequence[Extension] | None = None,
                 auth_server_provider: OAuthAuthorizationServerProvider | None = None,
                 token_verifier: TokenVerifier | None = None,
                 lifespan: Callable | None = None,
                 auth: AuthSettings | None = None): ...
    
    def tool(self, name: str | None = None, *, title: str | None = None,
             description: str | None = None, annotations: ToolAnnotations | None = None,
             icons: list[Icon] | None = None) -> Callable:
        """Decorator to register a function as an MCP tool."""
        ...
    
    def resource(self, uri: str, *, name: str | None = None, ...) -> Callable:
        """Decorator to register a function as an MCP resource."""
        ...
    
    def prompt(self, name: str | None = None, *, description: str | None = None) -> Callable:
        """Decorator to register a function as an MCP prompt."""
        ...
    
    async def run(self, *, transport: Literal["stdio", "sse"] = "stdio") -> None:
        """Start the server."""
        ...

# Tool — internal tool registration
class Tool(BaseModel):
    fn: Callable[..., Any]              # The actual function
    name: str                           # Tool name
    title: str | None                   # Human-readable title
    description: str                    # What the tool does
    parameters: dict[str, Any]          # JSON Schema
    fn_metadata: FuncMetadata           # Function metadata + Pydantic model
    is_async: bool                      # Async detection
    context_kwarg: str | None           # Context injection parameter
    annotations: ToolAnnotations | None # Tool annotations (read-only, destructive, etc.)
    
    @classmethod
    def from_function(cls, fn: Callable, name: str | None = None, *,
                      description: str | None = None, context_kwarg: str | None = None,
                      annotations: ToolAnnotations | None = None,
                      structured_output: bool | None = None) -> Tool:
        """Create a Tool from a function. Extracts schema from type hints."""
        ...
    
    async def run(self, arguments: dict[str, Any], context: Context, *,
                  convert_result: bool = False) -> Any:
        """Execute the tool with validated arguments."""
        ...

# FuncMetadata — extracts JSON Schema from function signatures
class FuncMetadata:
    arg_model: type[BaseModel]          # Pydantic model for arguments
    output_schema: dict[str, Any] | None  # Output JSON Schema
    
    async def call_fn_with_arg_validation(self, fn: Callable, is_async: bool,
                                           arguments: dict[str, Any],
                                           pass_directly: dict | None = None) -> Any: ...
    def validate_arguments(self, arguments: dict[str, Any]) -> dict[str, Any]: ...
```

### Client Side

```python
# Client — unified MCP client (v2 API)
@dataclass
class Client:
    server: Server | MCPServer | str | Transport  # Server instance or URL
    *, raise_exceptions: bool = False
    
    async def __aenter__(self) -> Client: ...
    async def list_tools(self) -> list[types.Tool]: ...
    async def call_tool(self, name: str, arguments: dict | None = None) -> types.CallToolResult: ...
    async def list_resources(self) -> list[types.Resource]: ...
    async def read_resource(self, uri: str) -> types.ReadResourceResult: ...
    async def list_prompts(self) -> list[types.Prompt]: ...
    async def get_prompt(self, name: str, arguments: dict | None = None) -> types.GetPromptResult: ...

# ClientSession — lower-level session management
class ClientSession:
    """Manages the JSON-RPC session with the server."""
    # Handles initialize handshake, capability negotiation, request/response

# ClientSessionGroup — manage multiple server connections
class ClientSessionGroup:
    """Group multiple MCP client sessions for parallel tool access."""
```

### Context Injection

```python
# Context parameter auto-injection
def find_context_parameter(fn: Callable) -> str | None:
    """Find parameter annotated with Context type in function signature."""
    # Scans function parameters for Context[...] type hint
    # Auto-injects server context (lifespan context, request context)

# Context — server context available to tools
class Context[LifespanContextT, RequestT]:
    session: ServerSession              # Current session
    lifespan_context: LifespanContextT   # Server lifespan context
    request_context: RequestT            # Current request context
    request_id: str                      # Current request ID
```

### Resolver System

```python
# Resolve — parameter resolvers for tool arguments
class Resolve:
    """Marker for parameters that should be resolved by the framework."""
    # Resolvers see validated arguments and can fill in additional context

def build_resolver_plans(resolved_params: dict, tool_arg_names: set) -> dict:
    """Build static resolver execution plans."""
    ...

async def resolve_arguments(resolved_params: dict, resolver_plans: dict,
                            pre_validated: dict, context: Context) -> dict:
    """Execute resolvers to fill in resolved parameters."""
    ...
```

---

## 3. MCP Go SDK Architecture

### Server

```go
// MCPServer — Go MCP server implementation
type MCPServer struct {
    resourcesMu            sync.RWMutex
    promptsMu              sync.RWMutex
    toolsMu                sync.RWMutex
    toolMiddlewareMu       sync.RWMutex
    notificationHandlersMu sync.RWMutex
    tasksMu                sync.RWMutex
    // ... more mutexes for thread safety
    
    resources              map[string]resourceEntry
    resourceTemplates      map[string]resourceTemplateEntry
    tools                  map[string]ServerTool
    prompts                map[string]ServerPrompt
    toolFilters            []ToolFilterFunc
    promptFilters          []PromptFilterFunc
    toolMiddleware         []ToolHandlerMiddleware
    resourceMiddleware     []ResourceHandlerMiddleware
    // ...
}

// Handler function types
type ToolHandlerFunc func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error)
type ResourceHandlerFunc func(ctx context.Context, request mcp.ReadResourceRequest) ([]mcp.ResourceContents, error)
type PromptHandlerFunc func(ctx context.Context, request mcp.GetPromptRequest) (*mcp.GetPromptResult, error)
type TaskToolHandlerFunc func(ctx context.Context, request mcp.CallToolRequest) (*mcp.CreateTaskResult, error)

// Middleware types
type ToolHandlerMiddleware func(ToolHandlerFunc) ToolHandlerFunc
type ResourceHandlerMiddleware func(ResourceHandlerFunc) ResourceHandlerFunc
type PromptHandlerMiddleware func(PromptHandlerFunc) PromptHandlerFunc

// Filter types — per-session tool/prompt filtering
type ToolFilterFunc func(ctx context.Context, tools []mcp.Tool) []mcp.Tool
type PromptFilterFunc func(ctx context.Context, prompts []mcp.Prompt) []mcp.Prompt

// Server options pattern
type ServerOption func(*MCPServer)

// Key types
type ServerTool struct {
    Tool    mcp.Tool
    Handler ToolHandlerFunc
}

type ServerTaskTool struct {
    Tool    mcp.Tool
    Handler TaskToolHandlerFunc  // Async task execution
}

// Context access
func ServerFromContext(ctx context.Context) *MCPServer {
    // Retrieve server instance from context
}
```

### Transport Implementations (Go)

```go
// Stdio transport
func NewStdioServer(server *MCPServer) *StdioServer
func (s *StdioServer) Listen(ctx context.Context, stdin io.Reader, stdout io.Writer) error

// SSE transport
func NewSSEServer(server *MCPServer, opts ...SSEOption) *SSEServer
func (s *SSEServer) ServeHTTP(w http.ResponseWriter, r *http.Request)

// Streamable HTTP transport
func NewStreamableHTTPServer(server *MCPServer, opts ...HTTPOption) *StreamableHTTPServer

// HTTP CORS support
func WithCORS(origins []string) ServerOption

// Task system for async tool execution
type taskEntry struct {
    task       mcp.Task
    sessionID  string
    toolName   string
    createdAt  time.Time
    result     any
    resultErr  error
    cancelFunc context.CancelFunc
    done       chan struct{}
    completed  bool
}
```

---

## 4. Function Calling Patterns

### go-openai Function Calling

```go
// Tool definition
type Tool struct {
    Type     string       `json:"type"`              // "function"
    Function *FunctionDefinition `json:"function"`
}

type FunctionDefinition struct {
    Name        string         `json:"name"`
    Description string         `json:"description,omitempty"`
    Parameters  jsonschema.Definition `json:"parameters"`
    Strict      bool           `json:"strict,omitempty"`
}

// Tool call in response
type ToolCall struct {
    ID       string       `json:"id"`
    Type     string       `json:"type"`
    Function FunctionCall `json:"function"`
}

type FunctionCall struct {
    Name      string `json:"name"`
    Arguments string `json:"arguments"`
}

// Tool message (result)
type ChatCompletionMessage struct {
    Role         string `json:"role"`
    Content      string `json:"content,omitempty"`
    ToolCalls    []ToolCall `json:"tool_calls,omitempty"`
    ToolCallID   string `json:"tool_call_id,omitempty"`
    FunctionCall *FunctionCall `json:"function_call,omitempty"`
}

// Chat completion request
type ChatCompletionRequest struct {
    Model    string                    `json:"model"`
    Messages []ChatCompletionMessage   `json:"messages"`
    Tools    []Tool                    `json:"tools,omitempty"`
    ToolChoice interface{}             `json:"tool_choice,omitempty"`
    Stream   bool                      `json:"stream,omitempty"`
    // ... temperature, max_tokens, etc.
}
```

### Streaming (go-openai)

```go
// Generic stream reader
type streamReader[T streamable] struct {
    emptyMessagesLimit uint
    isFinished         bool
    reader             *bufio.Reader
    response           *http.Response
    errAccumulator     utils.ErrorAccumulator
    unmarshaler        utils.Unmarshaler
}

func (stream *streamReader[T]) Recv() (response T, err error) {
    // Read next chunk from SSE stream
    // Handles: data: prefix, [DONE] marker, error accumulation
}

func (stream *streamReader[T]) processLines() ([]byte, error) {
    // Parse SSE lines:
    // 1. Read line from buffered reader
    // 2. Check for error prefix
    // 3. Strip "data: " prefix
    // 4. Check for "[DONE]" marker
    // 5. Return parsed JSON bytes
}
```

### Client Configuration (go-openai)

```go
// Multi-provider client config
type ClientConfig struct {
    authToken            string
    BaseURL              string
    OrgID                string
    APIType              APIType        // OPEN_AI, AZURE, AZURE_AD, ANTHROPIC
    APIVersion           string
    AssistantVersion     string
    AzureModelMapperFunc func(model string) string
    HTTPClient           HTTPDoer
    EmptyMessagesLimit   uint
}

// Provider presets
func DefaultConfig(authToken string) ClientConfig              // OpenAI
func DefaultAzureConfig(apiKey, baseURL string) ClientConfig   // Azure
func DefaultAnthropicConfig(apiKey, baseURL string) ClientConfig // Anthropic

// API types
const (
    APITypeOpenAI          APIType = "OPEN_AI"
    APITypeAzure           APIType = "AZURE"
    APITypeAzureAD         APIType = "AZURE_AD"
    APITypeCloudflareAzure APIType = "CLOUDFLARE_AZURE"
    APITypeAnthropic       APIType = "ANTHROPIC"
)
```

---

## 5. Model Routing Strategies (LiteLLM)

### Router Architecture

```python
class Router:
    def __init__(self, model_list: list[DeploymentTypedDict] | None = None,
                 # Caching
                 redis_url: str | None = None, cache_responses: bool = False,
                 caching_groups: list[tuple] | None = None,
                 # Reliability
                 num_retries: int | None = None, timeout: float | None = None,
                 fallbacks: list = [], context_window_fallbacks: list = [],
                 # Routing
                 routing_strategy: Literal[
                     "simple-shuffle", "least-busy", "usage-based-routing",
                     "latency-based-routing", "cost-based-routing",
                     "usage-based-routing-v2", "lar1"
                 ] = "simple-shuffle",
                 routing_groups: list[RoutingGroup] | None = None,
                 # Health
                 enable_health_check_routing: bool = False,
                 allowed_fails: int | None = None,
                 cooldown_time: float | None = None,
                 # Rate limiting
                 enable_pre_call_checks: bool = False,
                 # Weighted failover
                 enable_weighted_failover: bool = False,
                 retry_policy: RetryPolicy | dict | None = None,
                 model_group_retry_policy: dict[str, RetryPolicy] = {},
                 **kwargs) -> None: ...

# Routing strategies
class RoutingStrategy:
    simple_shuffle      # Random selection from available deployments
    least_busy          # Route to deployment with fewest active requests
    usage_based_routing # Route based on TPM/RPM usage
    latency_based_routing # Route to lowest latency deployment
    cost_based_routing  # Route to cheapest deployment
    adaptive_routing    # ML-based adaptive routing
    complexity_routing  # Route based on prompt complexity
    quality_routing     # Route based on historical quality scores

# Deployment definition
class DeploymentTypedDict(TypedDict):
    model_name: str                 # Model alias (e.g., "gpt-4")
    litellm_params: LiteLLM_Params  # Actual model config
    model_info: ModelInfo | None    # Model metadata

class LiteLLM_Params(TypedDict):
    model: str                      # "azure/gpt-4", "openai/gpt-4", etc.
    api_key: str | None
    api_base: str | None
    api_version: str | None
    rpm: int | None                 # Rate limit
    tpm: int | None                 # Token limit
    max_retries: int | None
    timeout: float | None
    # ... more params
```

### Cooldown & Health Management

```python
# Cooldown system — temporarily remove failing deployments
class CooldownCache:
    async def _get_cooldown_deployments(self) -> list[str]: ...
    async def _set_cooldown_deployments(self, deployment: str, 
                                         cooldown_time: float) -> None: ...

# Health check routing
class DeploymentHealthCache:
    """Tracks deployment health status for routing decisions."""
    # Health checks run at configurable intervals
    # Staleness threshold determines when to re-check
    # Transient errors can be optionally ignored

# Pre-call checks
class DeploymentAffinityCheck:     # Sticky sessions to deployments
class ModelRateLimitingCheck:      # TPM/RPM enforcement
class PromptCachingDeploymentCheck: # Route to caching-enabled deployments
```

### Cost Tracking

```python
# Cost calculator — per-provider cost calculation
class CostCalculator:
    @staticmethod
    def cost_per_token(model: str, usage: Usage, 
                       custom_pricing: dict | None = None) -> tuple[float, float]:
        """Returns (prompt_cost, completion_cost) in USD."""
        ...
    
    @staticmethod
    def completion_cost(model: str, messages: list, response: ModelResponse) -> float:
        """Calculate total cost for a completion call."""
        ...

# Provider-specific cost calculators:
# - openai_cost_per_token
# - anthropic_cost_per_token  
# - azure_openai_cost_per_token
# - google_cost_per_token (Vertex AI)
# - bedrock_cost_per_token
# - deepseek_cost_per_token
# - fireworks_ai_cost_per_token
# - perplexity_cost_per_token
# - xai_cost_per_token

# Budget management
class RouterBudgetLimiting:
    """Enforce budget limits per provider/model."""
    # Daily/monthly budget tracking
    # Per-provider limits
    # Alert thresholds
```

### Retry & Fallback Logic

```python
# Retry policy
class RetryPolicy:
    ContentPolicyViolationError: int | None  # Custom retry count per error
    RateLimitError: int | None
    TimeoutError: int | None
    # ...

# Fallback chain
# 1. Try primary deployment
# 2. On failure → try other deployments in same model group (weighted failover)
# 3. On group exhaustion → try cross-group fallbacks
# 4. On fallback exhaustion → try content_policy_fallbacks / context_window_fallbacks

# Headers for tracing
def add_retry_headers_to_response(response, retry_count, deployment_name): ...
def add_fallback_headers_to_response(response, fallback_model): ...
```

---

## 6. LLM Client Abstraction (Continue)

### BaseLLM Architecture

```typescript
// BaseLLM — abstract LLM client
abstract class BaseLLM implements ILLM {
    static providerName: string;
    static defaultOptions: Partial<LLMOptions> | undefined;
    
    // Core properties
    uniqueId: string;
    model: string;
    title?: string;
    apiBase?: string;
    apiKey?: string;
    completionOptions: CompletionOptions;
    requestOptions?: RequestOptions;
    template?: TemplateType;
    capabilities?: ModelCapability;
    roles?: ModelRole[];
    
    // Capability detection
    supportsFim(): boolean;           // Fill-in-the-middle
    supportsImages(): boolean;        // Vision
    supportsCompletions(): boolean;   // Legacy completions endpoint
    supportsPrefill(): boolean;       // Prefill support
    
    // Core methods
    streamChat(messages: ChatMessage[], options: LLMFullCompletionOptions,
               abortSignal?: AbortSignal): AsyncGenerator<ChatMessage>;
    chatCompletionNonStream(params: ChatCompletionCreateParams): Promise<ChatCompletion>;
    streamComplete(prompt: string, options: CompletionOptions): AsyncGenerator<string>;
    complete(prompt: string, options: CompletionOptions): Promise<string>;
    
    // Token counting
    countTokens(text: string): Promise<number>;
    countChatMessageTokens(message: ChatMessage): Promise<number>;
    
    // FIM (Fill-in-the-Middle)
    streamFim(prefix: string, suffix: string, options: CompletionOptions): AsyncGenerator<string>;
    
    // Model info
    listModels(): Promise<string[]>;
}

// Provider detection
function autodetectTemplateType(model: string): TemplateType | undefined;
function autodetectPromptTemplates(model: string, templateType: TemplateType): Record<string, PromptTemplate>;
function modelSupportsImages(providerName: string, model: string, ...): boolean;

// Token counting
function countTokens(model: string, text: string, ...): number;
function compileChatMessages(model: string, messages: ChatMessage[], maxTokens: number): ChatMessage[];

// Type conversion (OpenAI format ↔ provider format)
function toChatBody(messages: ChatMessage[], options: CompletionOptions): ChatCompletionCreateParams;
function fromChatResponse(response: ChatCompletion): ChatMessage;
function fromChatCompletionChunk(chunk: ChatCompletionChunk): ChatMessage | undefined;
```

### Context Providers (Continue)

```typescript
// BaseContextProvider — inject context into LLM prompts
abstract class BaseContextProvider implements IContextProvider {
    options: { [key: string]: any };
    static description: ContextProviderDescription;
    
    abstract getContextItems(query: string, extras: ContextProviderExtras): Promise<ContextItem[]>;
    async loadSubmenuItems(args: LoadSubmenuItemsArgs): Promise<ContextSubmenuItem[]>;
}

// Built-in providers:
// - CodeContextProvider — selected code blocks
// - CodebaseContextProvider — semantic code search
// - CurrentFileContextProvider — active file contents
// - DatabaseContextProvider — SQL query results
// - DebugLocalsProvider — debugger variables
// - ClipboardContextProvider — clipboard contents
// - CustomContextProvider — user-defined providers

// MCP integration
class MCPConnection {
    /* Manages MCP server connection for tool/context access */
}
class MCPManagerSingleton {
    /* Singleton managing multiple MCP server connections */
}
```

### Streaming Implementation

```typescript
// StreamChat — core streaming implementation
async function* streamChat(
    llm: BaseLLM,
    messages: ChatMessage[],
    options: LLMFullCompletionOptions,
    abortSignal?: AbortSignal
): AsyncGenerator<ChatMessage> {
    // 1. Build request with proper format conversion
    // 2. Handle streaming chunks via SSE
    // 3. Aggregate tool calls across chunks
    // 4. Yield complete messages
    // 5. Handle errors with exponential backoff
}

// Exponential backoff for retries
async function withExponentialBackoff<T>(
    fn: () => Promise<T>,
    maxRetries: number = 3,
    baseDelay: number = 1000
): Promise<T>;
```

---

## 7. Hidden Gems

### MCP Python SDK
- **Resolver system** — `@Resolve` decorator auto-fills tool parameters from context
- **Structured output** — `structured_output=True` generates output JSON Schema from return type
- **Tool annotations** — `ToolAnnotations` marks tools as `readOnlyHint`, `destructiveHint`, `openWorldHint`
- **Context injection** — `Context[...]` type hint auto-injects server context into tool functions
- **Direct dispatcher** — `DirectDispatcher` bypasses JSON-RPC framing for in-process calls
- **OAuth support** — Full OAuth 2.0 authorization server with `OAuthAuthorizationServerProvider`
- **Extensions** — `Extension` class for composable server middleware
- **Session groups** — `ClientSessionGroup` manages multiple MCP server connections
- **Input required** — `InputRequiredResult` for interactive tool workflows

### MCP Go SDK
- **Task tools** — `TaskToolHandlerFunc` for async tool execution with task tracking
- **Tool middleware** — `ToolHandlerMiddleware` chain for cross-cutting concerns
- **Tool filters** — Per-session tool visibility via `ToolFilterFunc`
- **Prompt filters** — Per-session prompt visibility via `PromptFilterFunc`
- **Protected resources** — OAuth-protected resource endpoints
- **CORS support** — Built-in CORS configuration for HTTP transports
- **Tracing** — OpenTelemetry tracing integration
- **Session hooks** — Lifecycle hooks for session management
- **Strict input validation** — JSON Schema validation for tool inputs
- **In-process sessions** — `InProcessSession` for testing without transport overhead

### LiteLLM
- **Adaptive routing** — ML-based routing that learns from request patterns
- **Complexity routing** — Route based on prompt complexity (token count, tool calls)
- **Quality routing** — Route based on historical response quality scores
- **Tag-based routing** — Route by metadata tags (team, environment)
- **Pattern matching** — Regex-based model name routing
- **Dual cache** — `DualCache` (in-memory + Redis) for distributed caching
- **Credential accessor** — Dynamic credential injection per request
- **Sensitive data masking** — `SensitiveDataMasker` for logging
- **Provider budget config** — Per-provider daily/monthly budget limits
- **Deployment affinity** — Sticky sessions to specific deployments

### go-openai
- **Reasoning content** — `ReasoningContent` field for DeepSeek reasoning models
- **Content filter results** — Azure content safety integration
- **Multi-content messages** — `MultiContent` for multimodal (text + images)
- **Azure model mapper** — `AzureModelMapperFunc` for deployment name translation
- **Generic stream reader** — `streamReader[T streamable]` with type-safe deserialization
- **Anthropic API support** — `DefaultAnthropicConfig` for Claude models

### Continue
- **FIM support** — Fill-in-the-middle for code completion
- **Template auto-detection** — `autodetectTemplateType` for provider-specific formatting
- **Token-aware pruning** — `compileChatMessages` respects context window limits
- **Tool overrides** — `applyToolOverrides` for per-model tool customization
- **Role-based model selection** — `ModelRole` (chat, autocomplete, edit, apply) with different models
- **MCP manager singleton** — `MCPManagerSingleton` for shared MCP connections
- **Exponential backoff** — Built-in retry with backoff for transient failures

---

## 8. Anti-Patterns

| Anti-Pattern | Impact | Fix |
|--------------|--------|-----|
| No transport abstraction | Vendor lock-in | Use `Transport` interface |
| Hardcoded API keys | Security risk | Use environment variables or secret managers |
| No retry logic | Transient failures kill requests | Implement exponential backoff |
| Ignoring streaming | Poor UX for long responses | Use `streamChat` / SSE |
| No cost tracking | Budget overruns | Track per-call costs via `CostCalculator` |
| Single model dependency | Single point of failure | Use Router with fallbacks |
| No health checks | Route to dead deployments | Enable `enable_health_check_routing` |
| Ignoring token limits | Context overflow errors | Count tokens, prune messages |
| Synchronous streaming | Event loop blocking | Use async generators |
| No tool validation | Invalid tool calls crash | Use JSON Schema validation |
| Mixing sync/async | Deadlocks | Pick one, use consistently |
| No middleware | Cross-cutting concerns duplicated | Use `ToolHandlerMiddleware` |
| Ignoring cooldowns | Hammer failing deployments | Respect `cooldown_time` |
| No logging | Silent failures | Use structured logging with request IDs |

---

## 9. Implementation Patterns

### Pattern: Tool Registration (MCP Python SDK)

```python
from mcp.server.mcpserver import MCPServer
from mcp.server.mcpserver.context import Context

server = MCPServer(name="my-tools")

@server.tool()
def search_database(query: str, limit: int = 10) -> list[dict]:
    """Search the database for matching records."""
    # Schema auto-extracted from type hints
    # Parameters validated against Pydantic model
    return db.search(query, limit=limit)

@server.tool(name="custom_name", annotations={"readOnlyHint": True})
async def fetch_data(url: str, ctx: Context) -> str:
    """Fetch data from URL. Context auto-injected via ctx parameter."""
    session = ctx.session  # Access MCP session
    return await http_client.get(url).text

# Run server
await server.run(transport="stdio")
```

### Pattern: Multi-Model Router (LiteLLM)

```python
from litellm import Router

router = Router(
    model_list=[
        {"model_name": "gpt-4", "litellm_params": {"model": "azure/gpt-4-deployment", "api_key": "..."}},
        {"model_name": "gpt-4", "litellm_params": {"model": "openai/gpt-4", "api_key": "..."}},
        {"model_name": "claude-3", "litellm_params": {"model": "anthropic/claude-3-opus", "api_key": "..."}},
    ],
    fallbacks=[{"gpt-4": "claude-3"}],
    routing_strategy="latency-based-routing",
    num_retries=3,
    timeout=30,
    enable_health_check_routing=True,
    allowed_fails=3,
    cooldown_time=60,
)

# Usage
response = await router.acompletion(
    model="gpt-4",
    messages=[{"role": "user", "content": "Hello"}],
)
```

### Pattern: Go MCP Server with Middleware

```go
package main

import (
    "context"
    "github.com/mark3labs/mcp-go/mcp"
    "github.com/mark3labs/mcp-go/server"
)

func main() {
    s := server.NewMCPServer("my-tools", "1.0.0",
        server.WithToolCapabilities(true),
    )

    // Add logging middleware
    s.UseToolMiddleware(func(next server.ToolHandlerFunc) server.ToolHandlerFunc {
        return func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
            log.Printf("Tool call: %s", req.Params.Name)
            return next(ctx, req)
        }
    })

    // Register tool
    tool := mcp.NewTool("search",
        mcp.WithDescription("Search the database"),
        mcp.WithString("query", mcp.Required(), mcp.Description("Search query")),
    )
    s.AddTool(tool, searchHandler)

    // Run stdio
    stdioServer := server.NewStdioServer(s)
    stdioServer.Listen(context.Background(), os.Stdin, os.Stdout)
}
```

### Pattern: Streamable HTTP Client (MCP Python)

```python
from mcp.client import Client

# Connect to remote MCP server
async with Client("https://mcp.example.com/tools") as client:
    tools = await client.list_tools()
    result = await client.call_tool("search", {"query": "test"})
    print(result)

# In-process testing
from mcp.server.mcpserver import MCPServer
server = MCPServer(name="test")
@server.tool()
def hello(name: str) -> str: return f"Hello {name}"

async with Client(server) as client:
    result = await client.call_tool("hello", {"name": "world"})
```

---

## 10. Verification Checklist

- [ ] MCP server capabilities declared correctly (tools, resources, prompts)
- [ ] Tool schemas validated (JSON Schema from type hints)
- [ ] Context injection used for session/lifespan data (not global state)
- [ ] Error handling returns proper MCP error responses (not Python exceptions)
- [ ] Streaming used for long-running operations
- [ ] Transport abstraction layer allows swapping stdio ↔ HTTP
- [ ] Auth configured for production deployments (OAuth 2.0 / Bearer tokens)
- [ ] Health checks enabled for model router deployments
- [ ] Retry policies configured with exponential backoff
- [ ] Cost tracking enabled for budget monitoring
- [ ] Fallback chains configured for high availability
- [ ] Token counting validates against context window limits
- [ ] Tool middleware used for cross-cutting concerns (logging, auth, validation)
- [ ] Cooldown configured for failing deployments
- [ ] Rate limiting enforced (TPM/RPM) per deployment
- [ ] Structured output schemas validated on both client and server
- [ ] MCP tool names follow naming conventions (no special characters)
- [ ] Resource subscriptions used for real-time updates
- [ ] Prompt templates versioned and tested
- [ ] Client session groups used for multi-server tool aggregation


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

- `ai-agent-orchestration`
- `context-engineering`

