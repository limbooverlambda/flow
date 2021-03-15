package workflow

import (
	"flow/steps"
)

type flow struct {
	name       string
	startsteps []steps.Step
}

//Name returns the name of the Workflow
func (f flow) Name() string {
	return f.name
}

//StartSteps returns the initial set of steps that will trigger the flow
func (f flow) StartSteps() []steps.Step {
	return f.startsteps
}

type step struct {
	name     string
	nextstep []steps.Step
}

//Name returns the name of the Step
func (s step) Name() string {
	return s.name
}

//NextSteps returns the next set of steps that need to be executed
func (s step) NextSteps() []steps.Step {
	return s.nextstep
}
