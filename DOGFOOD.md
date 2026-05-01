# llm-recall — Dogfood Log

> W4 后半周 + W5 用户实战周。每日 ≥ 1 条 entry。一周 ≥ 5 条进入 W5。

## Format

```
[YYYY-MM-DD][bug | impr | obs] <短描述> -- <复现/上下文> -- <action: fix-now | W5+ | wontfix>
```

## Entries

- [2026-05-01][obs] W4 boot entry: `goreleaser release --snapshot --clean` 本地跑通，5 个 archive 就位；release.yml YAML 合法。等用户配 GitHub Secrets + 推 tag v0.1.0 触发真发版。 -- action: 用户手动
- [2026-05-01][impr] W5-rev1: PNG 生图链路废，改终端原生 stats（用户驳回 3 张候选模板，要求 bashtop 风格）。strategy P0-5 不变（截屏发朋友圈仍然成立）。

(以下由用户填)

- [ ][bug | impr | obs] ...
