# Sprint 52 — Output Streaming: executor em tempo real (v2.0.0)

> **Status**: Shipped ✅  
> **Version target**: v2.0.0 (breaking change de UX: output ao vivo)

---

## Background

`loop.Run()` usava `SimpleChat` para o executor — a resposta só aparecia após o
LLM terminar. Em goals complexos isso significa minutos de silêncio. Sprint 52
adiciona streaming opt-in: com `--stream`, cada token do executor é escrito no
stdout conforme chega.

---

## O que foi construído

### `internal/loop/runner.go`

**`RunConfig.Stream bool`** — quando `true`, o executor usa `ChatStream`.
Verifier e reviewer continuam com `SimpleChat` — o output deles é parseado
programaticamente, não exibido.

**`RunConfig.StreamOut StreamWriter`** — writer para os chunks. `nil` → `os.Stdout`.
Injetável em testes para capturar output sem printar.

**`StreamWriter` interface** — `Write(p []byte) (n int, err error)`. Satisfeita por
`*os.File`, `*bytes.Buffer`, qualquer `io.Writer`. Definida no package para evitar
dependência circular com `io`.

**`simpleChatStream(ctx, client, systemPrompt, userPrompt, w)`** — wrapper que:
1. Constrói `[]Message{system, user}` como `SimpleChat`
2. Chama `client.ChatStream(ctx, messages, callback)`
3. O callback acumula em `strings.Builder` e escreve em `w` chunk a chunk
4. Retorna o texto acumulado + erro — mesma assinatura que `SimpleChat`

**Fluxo por iteração quando `Stream=true`:**
```
fmt.Fprintf(streamOut, "\n── executor (iter N) ──────────────────────────────\n")
simpleChatStream(...)   ← chunks escritos em tempo real
fmt.Fprintf(streamOut, "\n────────────────────────────────────────────────────\n")
```

### Bug fix: `discover → discover` (capturado pelos testes)

O startup de `Run()` já faz `Transition(PhaseDiscover, "run started")`. O loop
interno tentava `Transition(PhaseDiscover, ...)` novamente na primeira iteração —
`invalid transition discover → discover`. Fix: skip da transição quando já em
`PhaseDiscover`.

Sem esse fix, `loop.Run()` retornaria erro em *toda* primeira chamada real.

### `cmd/radiant/main.go`

**`--stream`** flag em `loopStartCmd`. Mapeia para `RunConfig.Stream`.

```bash
radiant loop start "refactor the scheduler" --model claude-sonnet-4-6 --stream
```

Output durante o run:

```
✓ Loop starting
  Run ID:  run-1751234567
  Goal:    refactor the scheduler
  Model:   claude-sonnet-4-6

── executor (iter 1) ──────────────────────────────
The scheduler currently uses a polling approach...
[tokens appear in real time]
────────────────────────────────────────────────────

✓ Loop finished
  Exit:       success
  Iterations: 1
  Elapsed:    23s
  Tokens:     4200
```

### `internal/loop/sprint52_test.go` — 8 novos testes

- `StreamWriter` satisfeita por `*bytes.Buffer`
- `RunConfig.Stream` / `StreamOut` defaults e assignability
- `simpleChatStream` callback com writer nil — acumula sem pânico
- `Run()` com `Stream=true` escreve header `executor (iter N)` no buffer
- `Run()` com `Stream=false` deixa buffer vazio
- `Run()` com `Stream=true` escreve separador `──────`

---

## Invariantes

- **Fail-same**: `simpleChatStream` retorna `(string, error)` idêntico a `SimpleChat` — o loop não distingue sucesso/falha de forma diferente
- **Verifier não streama nunca** — o output do verifier é parseado; exibi-lo quebraria a UX
- **StreamOut=nil → os.Stdout** — comportamento padrão seguro
- **Bug fix incluído**: `discover → discover` era um crash silencioso na primeira iteração real

---

## Referências

- `internal/llm/client.go` — `ChatStream(ctx, []Message, StreamCallback)`
- `internal/loop/runner.go` — `simpleChatStream`, `StreamWriter`, `RunConfig.Stream`
