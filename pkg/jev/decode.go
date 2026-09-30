package jev

import (
	"encoding/json"
	"fmt"
	"maps"
	"math"
	"strings"

	"google.golang.org/protobuf/proto"

	jevv1 "github.com/sudorandom/protoc-gen-jev/pkg/jev/v1"
)

// Decisions holds validated protobuf answers for generated clients to assign
// to their declared response fields. No application-message reflection is used.
type Decisions struct {
	Choices  map[string]*jevv1.Choice
	Nouls    map[string]*jevv1.Noul
	Scores   map[string]*jevv1.Score
	Meta     *jevv1.Meta
	Response *jevv1.Response
}

// Decode deserializes the provider response once and evaluates its decisions.
func Decode(body []byte, rules []Question) (*Decisions, error) {
	var wire jevv1.WireResponse
	if err := json.Unmarshal(body, &wire); err != nil {
		return nil, fmt.Errorf("failed to decode Jev response: %w", err)
	}
	out := &Decisions{
		Choices: make(map[string]*jevv1.Choice),
		Nouls:   make(map[string]*jevv1.Noul),
		Scores:  make(map[string]*jevv1.Score),
		Meta:    (jevv1.Meta_builder{Model: wire.GetModel(), Usage: wire.GetUsage()}).Build(),
	}
	for _, q := range rules {
		answer, err := findAnswer(&wire, q)
		if err != nil {
			return nil, err
		}
		invalid := func() (*Decisions, error) {
			return nil, fmt.Errorf("invalid %s answer for question %q", q.Type, q.Name)
		}
		switch q.Type {
		case "choice":
			if _, ok := q.Choices[answer.GetChoice()]; !ok || answer.GetChoice() == "" {
				return nil, fmt.Errorf("question %q: unknown choice %q", q.Name, answer.GetChoice())
			}
			out.Choices[q.Name] = (jevv1.Choice_builder{Value: answer.GetChoice(), Confidence: proto.ValueOrNil(answer.HasConfidence(), answer.GetConfidence), Probabilities: answer.GetProbabilities()}).Build()
		case "noul":
			if !answer.HasNoul() || !inRange(answer.GetNoul(), 0, 1) {
				return invalid()
			}
			out.Nouls[q.Name] = (jevv1.Noul_builder{Value: answer.GetNoul() >= q.Threshold, Probability: answer.GetNoul(), Confidence: proto.ValueOrNil(answer.HasConfidence(), answer.GetConfidence)}).Build()
		case "score":
			if !answer.HasScore() || len(q.Levels) < 2 || !inRange(answer.GetScore(), 0, float64(len(q.Levels)-1)) {
				return invalid()
			}
			position := answer.GetScore()
			index := int(position)
			value := q.Levels[index].Value
			if index+1 < len(q.Levels) {
				fraction := position - float64(index)
				value = (1-fraction)*value + fraction*q.Levels[index+1].Value
			}
			if strings.HasPrefix(q.Kind, "int") || strings.HasPrefix(q.Kind, "uint") {
				value = math.Round(value)
			}
			if !validScore(value, q.Kind) {
				return invalid()
			}
			out.Scores[q.Name] = (jevv1.Score_builder{Value: value, Score: position, Confidence: proto.ValueOrNil(answer.HasConfidence(), answer.GetConfidence), Probabilities: answer.GetProbabilities(), Legend: answer.GetLegend()}).Build()
		default:
			return invalid()
		}
	}
	if wire.GetAnswers() == nil {
		wire.SetAnswers(make(map[string]*jevv1.Answer))
		maps.Copy(wire.GetAnswers(), wire.GetChoices())
		maps.Copy(wire.GetAnswers(), wire.GetScores())
		maps.Copy(wire.GetAnswers(), wire.GetNouls())
	}
	out.Response = (jevv1.Response_builder{Model: wire.GetModel(), Usage: wire.GetUsage(), Answers: wire.GetAnswers()}).Build()
	return out, nil
}

func inRange(value, min, max float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0) && value >= min && value <= max
}

func validScore(value float64, kind string) bool {
	switch kind {
	case "int32":
		return inRange(value, math.MinInt32, math.MaxInt32)
	case "uint32":
		return inRange(value, 0, math.MaxUint32)
	case "int64":
		return inRange(value, -(1<<53 - 1), 1<<53-1)
	case "uint64":
		return inRange(value, 0, 1<<53-1)
	case "float32":
		return inRange(value, -math.MaxFloat32, math.MaxFloat32)
	default:
		return !math.IsNaN(value) && !math.IsInf(value, 0)
	}
}

func findAnswer(wire *jevv1.WireResponse, q Question) (*jevv1.Answer, error) {
	answers := wire.GetAnswers()
	canonical := answers != nil
	if !canonical {
		switch q.Type {
		case "choice":
			answers = wire.GetChoices()
		case "noul":
			answers = wire.GetNouls()
		case "score":
			answers = wire.GetScores()
		}
	}
	answer := answers[q.Name]
	if answer == nil {
		return nil, fmt.Errorf("missing or invalid answer for question %q", q.Name)
	}
	if (canonical || answer.GetType() != "") && answer.GetType() != q.Type {
		return nil, fmt.Errorf("question %q: expected answer type %q", q.Name, q.Type)
	}
	return answer, nil
}
