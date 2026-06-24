# Roadmap — Multi-Platform & Multi-LLM

> Backlog estruturado para a próxima sprint. Tudo aqui foi escrito pensando
> em **vendor neutrality** (nenhum provedor é privilegiado) e
> **cross-platform** (Linux, macOS, Windows; amd64, arm64).

## Sprint 4 — done (2026-06-24)

- [x] Wire cost display into `radiant run` final summary
- [x] Rate-limit awareness (429 → Retry-After honored)
- [x] Homebrew + Scoop + AUR manifests (distribution ready)
- [x] 157 tests passing, zero races

## Sprint 5 — up next

## Princípios

1. **Presets são opcionais, não obrigatórios.** Qualquer modelo OpenAI-
   compatible funciona via `--model=...` sem precisar estar na lista.
2. **Detecção é alfabética.** Sem viés "Claude first" — só porque alguém
   instalou o CLI não significa que o harness prefere.
3. **Plataforma detectada em runtime.** PATH, $HOME, env vars. Nada de
   hardcoded `/usr/local/bin` ou `~/Library`.
4. **Sem SDKs pesados.** HTTP puro via `net/http`. Adicionar um provedor
   novo é uma entrada em `PresetModels` + (se preciso) um `baseURL()`.

---

## Multi-LLM — adicionar provedores

### 1. Mistral / Codestral

- Adicionar em `PresetModels`:
  - `mistral-large-2` (OpenRouter: `mistralai/mistral-large-2`)
  - `codestral-22b` (OpenRouter: `mistralai/codestral-22b`, code-specialized)
- Vantagem: latency baixo na Europa, preços agressivos.
- Esforço: 30 min.

### 2. Cohere Command R+

- Adicionar em `PresetModels`:
  - `command-r-plus` (OpenRouter: `cohere/command-r-plus`)
- Forte em RAG (útil pro `/integracoes` e `/mapear`).
- Esforço: 15 min.

### 3. Groq (latência ultra-baixa)

- Adicionar provider `ProviderGroq` com baseURL `https://api.groq.com/openai/v1`.
- Adicionar presets:
  - `llama-3.3-70b` (Groq: `llama-3.3-70b-versatile`)
  - `mixtral-8x7b` (Groq: `mixtral-8x7b-32768`)
- Vantagem: ~300 tokens/segundo para inferência rápida em CI.
- Esforço: 1h (incluindo testes).

### 4. xAI Grok

- Adicionar provider `ProviderXAI` com baseURL `https://api.x.ai/v1`.
- Adicionar `grok-2` preset.
- Esforço: 30 min.

### 5. Together AI / Fireworks / OpenRouter (já tem)

- OpenRouter já funciona. Together/Fireworks são OpenAI-compatible —
  basta adicionar provider com baseURL custom e presets.
- Esforço: 1h cada.

### 6. Native Anthropic Messages API

- O client atual assume OpenAI-compatible shape (`/chat/completions`).
- Anthropic tem shape diferente (`/messages` com `system`, `messages`).
- Implementar adapter separado em `internal/llm/anthropic.go` quando o
  provedor for `ProviderAnthropic`. Usar header `x-api-key` em vez de
  `Authorization: Bearer`.
- Esforço: 4h (incluindo testes com httptest).

### 7. Streaming paralelo multi-provider

- Quando `radiant run` invoca múltiplos modelos em paralelo (research
  com Opus, implementation com Sonnet), hoje cada chamada é seq dentro
  do orchestrator.
- Adicionar `--parallel-models` flag que dispara N requests simultâneas
  e vota/consolida respostas.
- Esforço: 6h. Requer agregação inteligente (não é só primeiro a
  responder — é "qual tem menos alucinação").

---

## Multi-Platform — hardening cross-OS

### 1. Detecção de PATH cross-platform

- Hoje `exec.LookPath` é usado direto, que já é cross-platform.
- MAS: o `agent.go` faz `LastIndexAny(base, "/\\")` — não considera
  Windows path quirks (drive letters `C:\`, UNC `\\server\share`).
- Adicionar helper `basename(path)` em `internal/harness/agent.go` que
  lida com `/`, `\`, drive letters, e UNC paths.
- Esforço: 1h.

### 2. Windows: console encoding

- `fmt.Printf` no Windows consert pode quebrar com acentos (PT-BR).
- Setar `cmd.Stdout = console.ConsoleWriter{...}` ou setar code page
  antes de imprimir.
- Workaround pragmático: usar `golang.org/x/text/encoding/charmap` pra
  converter UTF-8 → CP850 quando GOOS=windows.
- Esforço: 2h.

### 3. Windows: shell `sh -c` não existe

- `runGate` e `engine.runGate` usam `sh -c <gate>`. No Windows não tem
  `sh` por padrão (tem Git Bash em alguns ambientes, mas não confiável).
- Detectar GOOS. No Windows, usar `cmd /c <gate>` ou exigir que o gate
  seja um .exe/.bat/.cmd direto (sem shell).
- Esforço: 4h. Requer testes cross-OS via `GOOS=windows go test`.

### 4. Windows: flock via LockFileEx

- `syscall.Flock` é Unix-only. No Windows, equivalente é
  `LockFileEx`/`UnlockFileEx` via golang.org/x/sys/windows.
- Adicionar build tag: `state_unix.go` (FLOCK) + `state_windows.go`
  (LockFileEx).
- Esforço: 4h.

### 5. macOS ARM64 (M1/M2/M3)

- Já compilado via goreleaser com `GOARCH=arm64`. Mas precisa testar:
  - `syscall.Flock` no macOS ARM — OK, mesmo BSD-style.
  - AppleScript nas skills de scaffold — verificar.
- Esforço: 2h (testes em runner arm64).

### 6. Linux: cgroups v2 awareness

- O harness spawna N goroutines pra paralelismo. Em containers com
  CPU shares limitados, isso pode saturar.
- Detectar `/sys/fs/cgroup/cpu.max` e auto-throttle o `MaxParallelTasks`.
- Esforço: 4h. Nice-to-have, não crítico.

### 7. Homebrew / Scoop / Chocolatey / apt formulas

- Distribuir via package managers acelera adoção.
- `brew install fortvna/tap/radiant` — formula simples.
- `scoop install radiant` — manifest JSON.
- `apt install radiant` — deb package via ghcr.io.
- Esforço: 1 dia por package manager.

---

## DX (Developer Experience)

### 1. `radiant models --provider=mistral`

- Listar modelos filtrados por provedor.
- `--all` mostra todos os provedores disponíveis.
- Esforço: 4h.

### 2. `radiant doctor`

- Diagnóstico de ambiente: PATH, $SHELL, version do Go usada pra
  build, agents detectados, providers disponíveis com chave API
  setada.
- Esforço: 4h.

### 3. `radiant config --global`

- Hoje o `--api-key` é per-run. Adicionar config persistente em
  `~/.config/radiant/config.yaml` (XDG) ou `~/Library/Application Support`
  (macOS) ou `%APPDATA%\Radiant` (Windows).
- Esforço: 4h.

### 4. Auto model routing

- Quando `radiant run` detecta fase "research" ou "plan", usar Opus
  / Sonnet-large automaticamente. Quando fase "implement", usar
  Sonnet/Haiku.
- Toggle via `--auto-route` flag.
- Esforço: 1 dia.

---

## Priorização sugerida

| # | Item | Esforço | Valor |
|---|---|---|---|
| 1 | Anthropic native client | 4h | ⭐⭐⭐ (Melhora DX com Claude direto) |
| 2 | Mistral / Codestral presets | 30min | ⭐⭐ (Mais opções) |
| 3 | Groq provider | 1h | ⭐⭐⭐ (Velocidade em CI) |
| 4 | Windows shell + flock | 4h + 4h | ⭐⭐⭐ (Adoção Windows) |
| 5 | `radiant doctor` | 4h | ⭐⭐ (DX) |
| 6 | Auto model routing | 1d | ⭐⭐ (Qualidade) |
| 7 | Homebrew tap | 1d | ⭐⭐⭐ (Distribuição) |
| 8 | Cohere / Grok presets | 45min | ⭐ (Mais opções) |

**Próximo sprint sugerido:** 1, 2, 3, 5, 7 — cobre provider diversity +
distribuição em 3-4 dias de trabalho focado.

---

## Métricas de sucesso

Ao final da próxima sprint:

- [ ] `go test ./...` passa em Linux/amd64, Linux/arm64, macOS/amd64,
      macOS/arm64, Windows/amd64
- [ ] `radiant doctor` roda nos 5 OSes e retorna OK
- [ ] Pelo menos 6 provedores LLM documentados com exemplo funcional
- [ ] Pelo menos 3 package managers oferecendo `radiant` (brew, scoop, apt)
- [ ] Nenhum comentário no código menciona "Claude first" ou "best for SDD"
- [ ] CI matrix expandida para incluir `windows-latest` e `macos-latest`
