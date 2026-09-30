package jevv1

import (
	"bytes"
	"encoding/json"
)

// UnmarshalJSON decodes provider answers without depending on generated Go fields.
func (x *Answer) UnmarshalJSON(data []byte) error {
	var wire struct {
		Type          string             `json:"type"`
		Choice        string             `json:"choice"`
		Noul          *noulProbability   `json:"noul"`
		Score         *float64           `json:"score"`
		Confidence    *float64           `json:"confidence"`
		Probabilities map[string]float64 `json:"probabilities"`
		Legend        map[string]string  `json:"legend"`
	}
	if err := json.Unmarshal(data, &wire); err != nil {
		return err
	}
	x.Reset()
	x.SetType(wire.Type)
	x.SetChoice(wire.Choice)
	if wire.Noul != nil {
		x.SetNoul(float64(*wire.Noul))
	}
	if wire.Score != nil {
		x.SetScore(*wire.Score)
	}
	if wire.Confidence != nil {
		x.SetConfidence(*wire.Confidence)
	}
	x.SetProbabilities(wire.Probabilities)
	x.SetLegend(wire.Legend)
	return nil
}

type noulProbability float64

func (p *noulProbability) UnmarshalJSON(data []byte) error {
	switch string(bytes.TrimSpace(data)) {
	case "true":
		*p = 1
	case "false":
		*p = 0
	default:
		return json.Unmarshal(data, (*float64)(p))
	}
	return nil
}

// UnmarshalJSON preserves the presence of canonical answers, even when the
// provider supplies null or an empty map. Invalid canonical answers must not
// fall back to the legacy choices/scores/nouls groups.
func (x *WireResponse) UnmarshalJSON(data []byte) error {
	var wire struct {
		Model   string             `json:"model"`
		Usage   *Usage             `json:"usage"`
		Answers canonicalAnswers   `json:"answers"`
		Choices map[string]*Answer `json:"choices"`
		Scores  map[string]*Answer `json:"scores"`
		Nouls   map[string]*Answer `json:"nouls"`
	}
	if err := json.Unmarshal(data, &wire); err != nil {
		return err
	}
	x.Reset()
	x.SetModel(wire.Model)
	x.SetUsage(wire.Usage)
	x.SetAnswers(wire.Answers)
	x.SetChoices(wire.Choices)
	x.SetScores(wire.Scores)
	x.SetNouls(wire.Nouls)
	return nil
}

// UnmarshalJSON decodes the provider's token usage into the opaque protobuf.
func (x *Usage) UnmarshalJSON(data []byte) error {
	var wire struct {
		InputTokens  int32 `json:"input_tokens"`
		OutputTokens int32 `json:"output_tokens"`
	}
	if err := json.Unmarshal(data, &wire); err != nil {
		return err
	}
	x.Reset()
	x.SetInputTokens(wire.InputTokens)
	x.SetOutputTokens(wire.OutputTokens)
	return nil
}

type canonicalAnswers map[string]*Answer

func (a *canonicalAnswers) UnmarshalJSON(data []byte) error {
	type plainAnswers map[string]*Answer
	if err := json.Unmarshal(data, (*plainAnswers)(a)); err != nil {
		return err
	}
	if *a == nil {
		*a = make(canonicalAnswers)
	}
	return nil
}
