# spec.md — 0001-resolva-o-case-de-modelagem-de-risco-de-cr-dito-menuflex-vaga-es

> Templated by `radiant-harness` self-driven mode (v3.6.0+) on 2026-06-30.
> Replace the [host-agent: fill in] markers with real acceptance criteria.

## Goal

```
Resolva o case de modelagem de risco de crédito MenuFlex (vaga Especialista em Modelagem de Risco de Crédito no iFood Pago). O case completo está em case_candidato.md e o dicionário dos dados em dicionario_dados.md. Os CSVs estão em data/menu_flex_historico.csv (1800 linhas, target default_90dpd_6m preenchido) e data/menu_flex_novas_propostas.csv (250 linhas, target vazio — são as decisões a tomar). Entregue: (1) problema de modelagem definido (target, leakage, split out-of-time); (2) diagnóstico dos dados (taxa default, missings, outliers, segmentos); (3) modelo PD defensável com AUC/KS, calibração e drivers principais; (4) LGD/EAD/EL observados; (5) política de decisão para as 250 novas propostas (aprovar/rejeitar/revisão + limites + cap de exposição); (6) stress simples (queda GMV, etc) + 5-8 indicadores de monitoramento; (7) recomendação executiva (lança/não lança + travas + maior risco). Use a skill credit-risk (bundled) como base metodológica.
```


## Acceptance criteria

- AC1: [host-agent: fill in — task_id=ca6bcc83dafb27d3 phase=plan] (high-level — refine below)
- AC2: [host-agent: fill in — task_id=ca6bcc83dafb27d3 phase=plan] (high-level — refine below)
- AC3: [host-agent: fill in — task_id=ca6bcc83dafb27d3 phase=plan] (high-level — refine below)

## Non-goals

- [host-agent: fill in — task_id=ca6bcc83dafb27d3 phase=plan] (sketch; expand)

## Profile

- standard

---
[host-agent: fill in — task_id=ca6bcc83dafb27d3 phase=plan]
