package cost

// ModelPrice holds per-million-token rates for a model.
type ModelPrice struct {
	InputPerM  float64 // USD per 1M input tokens
	OutputPerM float64 // USD per 1M output tokens
}

// Prices maps model name to pricing. Values from published rates (April 2025).
var Prices = map[string]ModelPrice{
	"claude-sonnet": {InputPerM: 3.00, OutputPerM: 15.00},
	"claude-haiku":  {InputPerM: 0.25, OutputPerM: 1.25},
	"claude-opus":   {InputPerM: 15.00, OutputPerM: 75.00},
	"gpt-4o":        {InputPerM: 2.50, OutputPerM: 10.00},
	"gpt-4o-mini":   {InputPerM: 0.15, OutputPerM: 0.60},
}

// CalcCost returns the USD cost for the given token counts and model.
func CalcCost(model string, inputTokens, outputTokens int) float64 {
	p, ok := Prices[model]
	if !ok {
		return 0
	}
	return (float64(inputTokens)/1_000_000)*p.InputPerM +
		(float64(outputTokens)/1_000_000)*p.OutputPerM
}
