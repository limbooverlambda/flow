package workflow

import (
	"errors"
	"log"

	"flow/db"
	"flow/models"
	"flow/steps"
	"flow/storage"
	"flow/trigger"
)

//FlowRunner executes arbitrary workflows
type FlowRunner interface {
	StartRun(accountID string, flow Flow, context models.FlowContext, handlerMap *steps.HandlerMap) (models.RunID, error)
	GetWorkflowRun(flowRunID models.RunID) (models.WorkflowRun, error)
}

//NewFlowRunner ...
func NewFlowRunner(dbCommon *db.Common) FlowRunner {
	workflowRunStore := storage.NewWorkflowRunStore(dbCommon)
	taskRunStore := storage.NewTaskRunStore(dbCommon)
	return flowrunner{
		idGen:            NewIDGenerator(),
		workflowRunStore: workflowRunStore,
		taskRunStore:     taskRunStore,
		fUnwrap:          NewFlowUnwrapper(workflowRunStore, taskRunStore),
	}
}

type flowrunner struct {
	idGen            Id
	workflowRunStore storage.WorkflowRunStore
	taskRunStore     storage.TaskRunStore
	fUnwrap          FlowUnwrapper
}

//StartRun will perform the following:
// Sync path:
// 1. Generate the runID for the workflow based on the context. (done)
// RunID needs to be idemponent, which essentially means same function with the same inputs should generate the same id.
// All the task runs fall under the same workflow run-id so the taskrunID will be the hash of the discrete step.
// So, if you hash the workflow and then hash the context and then concatente them, the chance of collision is reduced.
// Should the accountID be exposed as a first class entity from the flow. This way the accountID can act as the differentiator.
//
// 2. Unroll the task graph (done), convert to taskRun (done) and persist the graph (done).
//  You get the start steps and then iterate over the next steps until the graph is persisted.
//  If the taskrunid is already present, skip, other wise persist the id along with its corresponding state.
//  Use optimistic-concurrency control to handle conflicting inserts.
// 3. Send the initial task(runs) + initial context to the supervisor
// 4. Persist the workflow run + initial context + start state (submitted).
//Async Path:
// WorkflowSupervisor:
// 1. Retrieve the queued up initial task + context
// 2. Wait until the workflow run is in submitted state.
// 3. Change the state of the run to running, if the state is already in running, terminate the routine.
// 4. Spawn TaskRun routines(send the task run id + context) for each of the tasks
// 5. Wait for completed TaskRunner routines or errors from the spawned routines.
// Happy path:
// 6. If the spawned routines are successful, merge the returned contexts with the original context, and pass it to subsequent tasks
// 7. If the spawned routines are successful, retrieve the next set of TaskRun ids and send the taskrunID + context to the taskrunner routine
// 8. Continue until there are no more next taskRunIDs
// 9. Mark the workflow runID as done and return from the supervisor.
// Failure:
// 10. Mark the workflow runID as failed and return from the supervisor.
//TaskRunner:
// 1. Retrieve the TaskRun payload for the taskrunid + context
// 2. Initialize the TaskRun state machine
// 3. Check the current state of the TaskRun.
// 4. Retrieve the step handler from the handlerMap.
// 5. Based on the current state of the TaskRun, mark the next state which will be either executing the task or keep checking the status.
// 6. If the status is in either unknown or failed, mark the state, send an error back and terminate the routine
// 7. If the status is done, mark the state as done and terminate the routine.
func (fr flowrunner) StartRun(accountID string, flow Flow, context models.FlowContext, handlerMap *steps.HandlerMap) (models.RunID, error) {
	//runID, err := fr.idGen.Generate(accountID, flow, context)
	//if err != nil {
	//	return "", err
	//}
	runID := fr.idGen.GenerateID()
	flowGraph, err := fr.fUnwrap.ToFlowGraph(runID, context, flow)
	if err != nil {
		return "", err
	}
	err = fr.validateFlowGraph(flowGraph)
	if err != nil {
		return "", err
	}
	fTrigger := trigger.NewFlowTrigger(fr.workflowRunStore, fr.taskRunStore)
	if err := fTrigger.StartRun(runID, handlerMap); err != nil {
		return "", err
	}
	log.Printf("Started the flow run with runID %v\n", runID)
	return models.RunID(runID), nil
}

func (fr flowrunner) GetWorkflowRun(flowRunID models.RunID) (models.WorkflowRun, error) {
	fTrigger := trigger.NewFlowTrigger(fr.workflowRunStore, fr.taskRunStore)
	return fTrigger.GetWorkflowRun(flowRunID)
}

func (fr flowrunner) validateFlowGraph(f models.Flowgraph) error {
	if len(f.Flowsteps) == 0 {
		return errors.New("unable to get flow steps for the graph")
	}
	return nil
}
