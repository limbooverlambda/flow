package steps

import (
	"flow/models"
)

//StepStatus is the status returned by the handler
//TODO: Generate the String representation for the StepStatus
type StepStatus int

const (
	//StepInProgress -> InProgress
	StepInProgress StepStatus = iota
	//StepDone -> Done
	StepDone
	//StepFailed -> Failed
	StepFailed
	//StepStatusUnknown -> Unable to retrieve Step status
	StepStatusUnknown
)

//HandlerMap is the alias from stepname to handler logic
type HandlerMap map[string]StepHandler

//Step is the discrete Step that will be executed by the runner
type Step interface {
	//Name returns the name of the Step
	Name() string
	//NextSteps returns the next set of steps that need to be executed
	NextSteps() []Step
}

//StepHandler contains the exectution and status check logic for the step
type StepHandler interface {
	//Name of the Step
	Name() string
	//Run runs the execution logic for the Step. Takes a FlowContext and decorates it with additional attributes
	Run(flowContext models.FlowContext) (models.FlowContext, error)
	//Checks the status of the step based on the current context
	Status(flowContext models.FlowContext) (StepStatus, error)
}
