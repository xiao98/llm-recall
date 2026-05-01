// Per-model pricing for the confirm-prompt cost estimate.
//
// Hardcoded because: (a) we ship two default models and any user-supplied
// override is one --model flag away; (b) live pricing fetch would require
// a network round-trip on every confirm, hurting offline UX; (c) the
// estimate is informational only — accurate to ±20% is fine.
//
// Rates are USD per million tokens, separated by direction (input vs
// output) because vendors charge asymmetrically.
//
// Last verified 2026-05. If the strict comparison shopper updates this,
// keep entries sorted vendor-then-model so diffs read clean.
package llm

import "fmt"

// pricing maps model id → (input USD/MTok, output USD/MTok).
type modelPrice struct {
	InputPerMTok  float64
	OutputPerMTok float64
}

var pricingTable = map[string]modelPrice{
	// Anthropic
	"claude-haiku-4-5-20251001":  {InputPerMTok: 1.00, OutputPerMTok: 5.00},
	"claude-3-5-haiku-20241022":  {InputPerMTok: 1.00, OutputPerMTok: 5.00},
	"claude-3-5-sonnet-20241022": {InputPerMTok: 3.00, OutputPerMTok: 15.00},
	"claude-sonnet-4-6":          {InputPerMTok: 3.00, OutputPerMTok: 15.00},
	// OpenAI
	"gpt-4o-mini": {InputPerMTok: 0.15, OutputPerMTok: 0.60},
	"gpt-4o":      {InputPerMTok: 5.00, OutputPerMTok: 15.00},
}

// EstimateCostUSD computes the USD cost of a hypothetical call with the
// given input/output token counts. Unknown models fall back to a flat
// ~$0.01/call rough estimate so the confirm prompt still surfaces a
// sensible number rather than $0 (which would feel suspicious).
func EstimateCostUSD(model string, inputToks, outputToks int) float64 {
	if p, ok := pricingTable[model]; ok {
		return float64(inputToks)/1_000_000*p.InputPerMTok +
			float64(outputToks)/1_000_000*p.OutputPerMTok
	}
	// Unknown model fallback: assume ~$1/MTok input, $5/MTok output (a
	// haiku-class envelope). 0.01 floor so users see a non-zero cost.
	rough := float64(inputToks)/1_000_000*1.00 + float64(outputToks)/1_000_000*5.00
	if rough < 0.01 {
		rough = 0.01
	}
	return rough
}

// FormatCostUSD prints a USD figure at sensible precision: 4 decimals
// for sub-cent costs, 3 decimals up to a dollar, 2 above.
func FormatCostUSD(usd float64) string {
	switch {
	case usd < 0.01:
		return fmt.Sprintf("$%.4f", usd)
	case usd < 1.0:
		return fmt.Sprintf("$%.3f", usd)
	default:
		return fmt.Sprintf("$%.2f", usd)
	}
}
