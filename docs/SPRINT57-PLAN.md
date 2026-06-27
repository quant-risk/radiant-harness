# Sprint 57 — Context Detector: múltiplas fontes de sinal (v2.5.0)

> **Status**: Shipped ✅  
> **Version target**: v2.5.0

Responde ao ponto 2 do assessment GLM 5.2: o Context Engine detectava
domínio apenas por imports (heurística frágil — "risk" no module path
basta para virar finance mesmo sem contexto relevante).

---

## O que foi construído

### Três novas fases em `Detect()` — `internal/context/detector.go`

**Phase 2b — `scanModulePath`** (peso 20 por hit)

Lê a linha `module` do `go.mod` e compara o path completo com
`domainKeywordPatterns`. O module path é uma declaração explícita de
identidade do projeto — alto sinal, alto peso.

```
github.com/quant-risk/portfolio-engine → DomainFinance +20
github.com/acme/ml-inference            → DomainML      +20
github.com/acme/defi-protocol           → DomainBlockchain +20
```

**Phase 2c — `scanDocs`** (peso 8 por hit, até 200 linhas por arquivo)

Lê README.md, CLAUDE.md, docs/README.md e README.rst. Documentação é
a fonte de intenção mais explícita — escrita por humanos para humanos,
raramente ambígua sobre o domínio do projeto.

**Phase 2d — `scanDirNames`** (peso 12 top-level, 8 internal/ / cmd/)

Verifica nomes de diretórios nos três níveis mais estruturais do projeto.
`internal/trading/` → finance, `cmd/defi-bridge/` → blockchain.

### `domainKeywordPatterns` — novo mapa

Separado de `domainImportPatterns` (que era focado em nomes de pacotes Go/Python).
`domainKeywordPatterns` usa termos de negócio e domínio que aparecem em
prose: `"fintech"`, `"deep-learning"`, `"smart-contract"`, `"devops"`.

### `internal/context/sprint57_test.go` — 13 novos testes

- `scanModulePath`: finance, ml, path genérico
- `scanDocs`: README finance, README ml, CLAUDE.md ops, README bate import fraco
- `scanDirNames`: internal/trading, top-level ml/, cmd/defi-bridge
- Multi-source agreement (module + README + dir → finance)
- Diretório vazio → general

---

## Hierarquia de sinais (acumulativos, maior wins)

| Fase | Fonte | Peso/hit |
|------|-------|---------|
| 2b | go.mod module path | 20 |
| 2d | diretório top-level | 12 |
| 1 | filesystem signals (hardhat.config, Cargo.toml…) | 8–20 |
| 2d | internal/ ou cmd/ subdir | 8 |
| 2c | README / CLAUDE.md | 8 |
| 2 | import scan (50 linhas/arquivo) | 5 |

---

## Placar GLM 5.2 após Sprint 57

| # | Ponto | Status |
|---|-------|--------|
| 1 | `main.go` 7k linhas | ✅ Sprint 53B+54 (36 linhas) |
| 2 | Detector de domínio frágil | ✅ Sprint 57 (module + docs + dirs) |
| 3 | Fleet não spawna processos reais | ✅ Sprint 56 (Dispatcher) |
| 4 | `estimateTokens` ruim para Unicode | ✅ Sprint 53B (rune/3.5) |
| 5 | Loop skip Planning sem LLM | ✅ Sprint 55 (--plan flag) |
| 6 | Deps minimalistas | strength, mantido |
| 7 | 298 testes em 94 arquivos | baseline, ↑ a cada sprint |
| 8 | Module path | informação, sem mudança necessária |
