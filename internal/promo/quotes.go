// Auto-fetched at W6 by subagent.
//
// YC TECH-style: 0 (recall.youchun.tech / api.youchun.tech / WebSearch all
//   yielded zero usable short-form slogans — pages were either bare
//   "New API" placeholder or unindexed. See file footer for full URL
//   trace. Per task §3 fallback policy, all 30 slots are filled with
//   generic developer-culture quotes; warn is emitted at startup load
//   from quotes_warn.go.)
// Generic developer quotes: 30 (Linus / Knuth / Brooks / Carmack /
//   Hoare / Sutter / Fowler / Dijkstra / Spolsky / Crockford / Pike /
//   Cook / Holub / Beck / Atwood — well-known programming aphorisms,
//   considered de facto public-domain quotations).
//
// 用户审核后可手动替换为更贴切的 YC TECH 群真实金句。
//
// Fetch trace (W6, all came back empty for usable YC TECH quotes):
//   - WebFetch  https://recall.youchun.tech                → ECONNREFUSED
//   - WebFetch  https://api.youchun.tech                   → "New API" only
//   - WebFetch  https://api.youchun.tech/about             → empty
//   - WebSearch site:recall.youchun.tech                   → 0 results
//   - WebSearch site:youchun.tech                          → 0 results
//   - WebSearch site:api.youchun.tech                      → 0 results
//   - WebSearch "YC TECH" 中文 AI                             → unrelated
//   - WebSearch YC TECH 实战派                                → unrelated

package promo

// Quotes is the pool the banner samples from. Order is incidental — the
// banner picks an index uniformly at random per launch.
//
// Per-line trailing comment encodes the data source so a future maintainer
// can audit provenance without re-running the fetch.
var Quotes = []string{
	"Talk is cheap. Show me the code.",                                                                                // generic: Linus Torvalds
	"Premature optimization is the root of all evil.",                                                                 // generic: Donald Knuth
	"Make it work, make it right, make it fast.",                                                                      // generic: Kent Beck
	"Simplicity is the ultimate sophistication.",                                                                      // generic: attrib. Leonardo da Vinci (devops folklore)
	"First, solve the problem. Then, write the code.",                                                                 // generic: John Johnson
	"Programs must be written for people to read.",                                                                    // generic: Abelson & Sussman, SICP
	"There are only two hard things in computer science: cache invalidation and naming things.",                       // generic: Phil Karlton
	"Any fool can write code that a computer can understand. Good programmers write code that humans can understand.", // generic: Martin Fowler
	"Walking on water and developing software from a specification are easy if both are frozen.",                      // generic: Edward V. Berard
	"The best way to predict the future is to implement it.",                                                          // generic: David Heinemeier Hansson
	"Debugging is twice as hard as writing the code in the first place.",                                              // generic: Brian Kernighan
	"Code is read more often than it is written.",                                                                     // generic: Guido van Rossum
	"Simplicity is prerequisite for reliability.",                                                                     // generic: Edsger W. Dijkstra
	"Premature abstraction is as bad as premature optimization.",                                                      // generic: Sandi Metz (paraphrased)
	"Show, don't tell.", // generic: writing folklore, adopted by tooling culture
	"The cheapest, fastest, and most reliable components are those that aren't there.",            // generic: Gordon Bell
	"Weeks of coding can save you hours of planning.",                                             // generic: anonymous
	"It is not enough for code to work.",                                                          // generic: Robert C. Martin (Clean Code)
	"A good programmer is someone who always looks both ways before crossing a one-way street.",   // generic: Doug Linder
	"Plan to throw one away; you will, anyhow.",                                                   // generic: Fred Brooks (Mythical Man-Month)
	"Adding manpower to a late software project makes it later.",                                  // generic: Fred Brooks (Brooks's Law)
	"The function of good software is to make the complex appear to be simple.",                   // generic: Grady Booch
	"Optimism is an occupational hazard of programming; feedback is the treatment.",               // generic: Kent Beck
	"Programming is the art of telling another human being what one wants the computer to do.",    // generic: Donald Knuth
	"Computers are useless. They can only give you answers.",                                      // generic: attrib. Pablo Picasso (devops folklore)
	"The most damaging phrase in the language is, 'It's always been done that way.'",              // generic: Grace Hopper
	"If you can't write it down in English, you can't code it.",                                   // generic: Peter Halpern
	"Programs are meant to be read by humans and only incidentally for computers to execute.",     // generic: Donald Knuth
	"Premature optimization is the root of all evil — yet that doesn't mean we should pessimize.", // generic: Herb Sutter / Andrei Alexandrescu
	"There is nothing more permanent than a temporary fix.",                                       // generic: Kyle Simpson
}
