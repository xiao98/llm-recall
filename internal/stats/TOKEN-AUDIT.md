# 三家 LLM CLI Token 字段实测（W5 §6）

实测日期：2026-05-01
样本：本机 `~/.claude/` `~/.codex/` `~/.gemini/` 内随机抽 3 条会话。

| 家 | 找到 / 部分 / 没找到 | 字段路径 | 聚合方式 |
|---|---|---|---|
| claude | 找到 | `message.usage.{input_tokens, output_tokens}` (assistant lines) | `sum(input_tokens + output_tokens)` over assistant rows |
| codex | 找到 | `payload.info.total_token_usage.total_tokens` (event_msg / token_count) | 取最后一个 `token_count` 事件的 `total_token_usage.total_tokens`（已是累计值） |
| gemini | 找到 | `messages[].tokens.{input, output}` 或 `.total` | `sum(total)` 或 `sum(input + output)` over messages |

下面每家给一段脱敏 jsonl 片段（仅字段结构）。

## claude

文件位置：`~/.claude/projects/<encoded-cwd>/<sessionId>.jsonl`
关键字段：`type=assistant` 行的 `message.usage.input_tokens` + `output_tokens`（外加 `cache_creation_input_tokens` / `cache_read_input_tokens`，但 cache 部分**不计**进 W5 总量，只算实付 input + output）。

```json
{"type":"assistant","message":{"usage":{
    "input_tokens": 3,
    "cache_creation_input_tokens": 4664,
    "cache_read_input_tokens": 11970,
    "output_tokens": 26
}}}
```

聚合：`sum_of_input + sum_of_output` 跨所有 assistant 行；不含 cache。

## codex

文件位置：`~/.codex/sessions/<YYYY>/<MM>/<DD>/rollout-*.jsonl`
关键字段：`type=event_msg` + `payload.type=token_count` 行；`payload.info.total_token_usage.total_tokens` 已经是 session 累计。

```json
{"type":"event_msg","payload":{
    "type":"token_count",
    "info":{
        "total_token_usage":{
            "input_tokens": 2516035,
            "cached_input_tokens": 2265344,
            "output_tokens": 54942,
            "reasoning_output_tokens": 23104,
            "total_tokens": 2570977
        }
    }
}}
```

聚合：取最后一个 `token_count` 事件的 `total_token_usage.total_tokens` 作为该 session 的总 token；session 之间求和。

## gemini

文件位置：`~/.gemini/tmp/<projectHash>/chats/session-*.json`（**单 JSON 不是 jsonl**）
关键字段：`messages[i].tokens.{input, output, total, cached, thoughts, tool}`；只有 `type=gemini` 的消息（即 model 输出）带 tokens，user 消息不带。

```json
{"messages":[
    {"id":"...","type":"user","content":"..."},
    {"id":"...","type":"gemini","tokens":{
        "input": 7772,
        "output": 157,
        "cached": 0,
        "thoughts": 1330,
        "tool": 0,
        "total": 9259
    }}
]}
```

聚合：`sum(messages[].tokens.total)` 跨所有 `tokens` 字段非空的消息（即 gemini 类型的消息）。每条消息的 `total` 已包含 input+output+thoughts。

## stats 命令实现策略

W5 §5 的 stats 聚合按上述方式累加。三家都找到 token 字段，所以**不需要 fallback 到 total_messages**——但 schema 里仍保留 `total_messages` 字段，供：

1. 某些异常 jsonl（解析失败 / 字段缺失）的 session 用条数兜底
2. 模板 v1/v2/v3 在 `total_tokens == 0` 时显示"消息条数"代替

**当前结论**：三家 token 都可用，stats 命令优先聚合 `total_tokens`；遇到旧版本 / 字段不存在时按 session 落到 `total_messages` 桶。
