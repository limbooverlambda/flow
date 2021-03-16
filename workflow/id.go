package workflow

import (
	"crypto/sha256"
	"fmt"
	"github.com/segmentio/ksuid"

	"github.com/mitchellh/hashstructure"

	"flow/models"
	"flow/steps"
)

const maxlen = 12

type Id interface {
	Generate(accountID string, flow Flow, context models.FlowContext) (string, error)
	GenerateID() string
}

func NewIDGenerator() Id {
	return runID{}
}

type runID struct{}

func (r runID) GenerateID() string {
	id := ksuid.New().String()
	var l int
	if len(id) >= maxlen {
		l = maxlen
	} else {
		l = len(id)
	}
	return id[:l]
}

func (r runID) Generate(accountID string, flow Flow, context models.FlowContext) (string, error) {
	id := sha256.New()
	_, err := id.Write([]byte(r.genFlowID(flow)))
	if err != nil {
		return "", err
	}
	ctxtID, err := r.genContextID(context)
	if err != nil {
		return "", err
	}
	_, err = id.Write([]byte(ctxtID))
	if err != nil {
		return "", err
	}
	_, err = id.Write([]byte(accountID))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", id.Sum(nil)), nil
}

func (r runID) genFlowID(f Flow) string {
	//f := r.flow
	var buffer []interface{}
	for _, ns := range f.StartSteps() {
		buffer = append(buffer, ns)
	}
	flowName := f.Name()
	id := sha256.New()
	id.Write([]byte(flowName))

	processFn := func(s interface{}) {
		step := s.(steps.Step)
		id.Write([]byte(step.Name()))
	}

	getNextFn := func(s interface{}) []interface{} {
		step := s.(steps.Step)
		var buffer []interface{}
		for _, ns := range step.NextSteps() {
			buffer = append(buffer, ns)
		}
		return buffer
	}
	bfs(buffer, processFn, getNextFn)
	return fmt.Sprintf("%x", id.Sum(nil))
}

func (r runID) genContextID(ctxt models.FlowContext) (string, error) {
	id, err := hashstructure.Hash(ctxt, nil)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%v", id), nil
}
