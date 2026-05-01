// Hardcoded mock outputs used by mockClient. Kept in a separate file so
// reviewers can scan them at a glance without wading through fixture
// strings inline in mock.go.
package llm

// MockCardFixture: canonical "what is the user doing" one-liner that a
// real model would produce given a developer chat session. Constraints:
//
//   - ≤ 50 chars (matches the card system-prompt requirement)
//   - Action verb start, no quotes, no markdown
//
// We pick a Chinese sentence so the rendering path that handles CJK
// width gets exercised in tests.
const MockCardFixture = "调试 sqlite cache 的 mtime 失效逻辑"

// MockGoldFixture: a JSON array of 10 quote/comment pairs. Must parse
// cleanly with the gold-command parser AND must be unique enough that
// a test can substring-match it in rendered output.
const MockGoldFixture = `[
  {"quote": "做事都搞到一半就停", "comment": "半成品=收益最大化位置"},
  {"quote": "AI 是这一代人的电力", "comment": "框架性判断，传播力强"},
  {"quote": "Talk is cheap. Show me the code.", "comment": "经典工程原则"},
  {"quote": "规约的演进只能由策划方 Edit 增量", "comment": "工程纪律的元规则"},
  {"quote": "claude code的历史会话管理太垃圾了", "comment": "用户原话槽点"},
  {"quote": "第一性原理：从原始需求出发", "comment": "决策心法"},
  {"quote": "动机不清晰时停下来讨论", "comment": "拒绝盲目行动"},
  {"quote": "遇到问题追根因，不打补丁", "comment": "工程态度"},
  {"quote": "输出说重点，砍掉一切不改变决策的信息", "comment": "信息密度优先"},
  {"quote": "show, don't tell", "comment": "传播原则"}
]`
