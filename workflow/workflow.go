package workflow

import (
	"flow/steps"
)

//Flow is the outline of the workflow executed by the runners
type Flow interface {
	//Name returns the name of the Workflow
	Name() string
	//StartSteps returns the initial set of steps that will trigger the flow
	StartSteps() []steps.Step
}




