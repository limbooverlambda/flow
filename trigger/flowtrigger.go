package trigger

import (
	"errors"
	"fmt"

	"flow/models"
	"flow/poll"
	"flow/steps"
	"flow/storage"
	"flow/tbsupervisor"
)

type Trigger interface {
	//Spawn the supervisor
	StartRun(runID string, handlerMap *steps.HandlerMap) error
	//Retrieve the flowRun
	GetWorkflowRun(flowRunID models.RunID) (models.WorkflowRun, error)
}

func NewFlowTrigger(workflowRunStore storage.WorkflowRunStore, taskRunStore storage.TaskRunStore) Trigger {
	return flowTrigger{
		wfRunStore: workflowRunStore,
		wfRunSupervisor: wfRunSupervisor{
			poller:       poll.NewWorkflowRunPoller(workflowRunStore),
			wfRunStore:   workflowRunStore,
			tRunStore:    taskRunStore,
			tbSupervisor: tbsupervisor.NewTaskBatchSupervisor(taskRunStore),
		},
	}
}

type workflowRunSupervisor interface {
	supervise(runID string, handlerMap *steps.HandlerMap) error
}

type flowTrigger struct {
	wfRunStore      storage.WorkflowRunStore
	wfRunSupervisor workflowRunSupervisor
}

func (f flowTrigger) StartRun(runID string, handlerMap *steps.HandlerMap) error {
	return f.wfRunSupervisor.supervise(runID, handlerMap)
}

func (f flowTrigger) GetWorkflowRun(flowRunID models.RunID) (models.WorkflowRun, error) {
	return f.wfRunStore.Get(string(flowRunID))
}

type wfRunSupervisor struct {
	poller       poll.WfRunPoller
	wfRunStore   storage.WorkflowRunStore
	tRunStore    storage.TaskRunStore
	tbSupervisor tbsupervisor.TaskBatchSupervisor
}

func (wfRun wfRunSupervisor) supervise(runID string, handlerMap *steps.HandlerMap) error {
	//TODO: This needs to be backed by a worker pool, if the pool limit is hit, there has to be a pushback
	go wfRun.supervisorFn(runID, handlerMap)
	return nil
}

//supervisorFn has the following responsibilities:
// Accepts a runID and a handlerMap that contains all the operations to be executed by the runner.
// - Polls for the presence of the runID
// - Checks for the status of the retrieved workflowRun
// - If the lifecycle is in SUBMITTED, flip the state to INPROGRESS
// - Update the status in the workflowRun store.
// - Retrieve the initial batch of tasks and send the taskBatch over to the taskBatchSupervisor
// - The taskBatchSupervisor returns two channels, a batchChannel and errorChannel.
// -   The batchChannel buffers the set of completed tasks and the error channel receives any errors from the tasks
// -   Upon receiving the set of completed tasks, the supervisorFn retrieves the next set of tasks and sends it over to the taskBatchSupervisor
//     until there are no more tasks.
// -   Upon receiving an error, the supervisor will exit.
func (wfRun wfRunSupervisor) supervisorFn(runID string, handlerMap *steps.HandlerMap) {
	var supervisorError error

	defer func() {
		if supervisorError != nil {
			fmt.Printf("Supervisor failed %v", supervisorError)
		}
		fmt.Printf("%v", "Workflow Done!, Exiting supervisor")
	}()

	run, err := wfRun.poller.Poll(runID)
	if err != nil {
		supervisorError = errors.New(fmt.Sprintf("Error while running supervisor %v", err))
		return
	}
	supervisorError = wfRun.processWorkflowRun(run, handlerMap)
	return
}

func (wfRun wfRunSupervisor) processWorkflowRun(run models.WorkflowRun, handlerMap *steps.HandlerMap) error {
	var err error
	switch run.Lifecycle {
	case models.SUBMITTED:
		{
			err = wfRun.claimWorkflowRun(run, handlerMap)
		}
	default:
		{
			err = errors.New(fmt.Sprintf("Illegal state for run %v", run))
		}
	}
	return err
}

func (wfRun wfRunSupervisor) claimWorkflowRun(run models.WorkflowRun, handlerMap *steps.HandlerMap) error {
	run.Lifecycle = models.CLAIMED
	claimedWfRun, err := wfRun.wfRunStore.Update(run)
	if err != nil {
		return err
	}
	return wfRun.progressWorkflowRun(claimedWfRun, handlerMap)
}

func (wfRun wfRunSupervisor) progressWorkflowRun(run models.WorkflowRun, handlerMap *steps.HandlerMap) error {
	taskRunBatch := tbsupervisor.TaskBatch{TaskRunIDs: run.InitialTaskRunIDs, Context: run.Context}
	batchChannel, errChannel := wfRun.tbSupervisor.Supervise(taskRunBatch, handlerMap)
	run.Lifecycle = models.INPROGRESS
	inprogressWfRun, err := wfRun.wfRunStore.Update(run)
	if err != nil {
		return wfRun.failWorkflowRun(inprogressWfRun, err)
	}
	var batchProcessingError error
workflowrun:
	for {
		select {
		case tb := <-batchChannel:
			{
				nextTaskRunBatch, err := wfRun.getNextTaskRunBatch(tb)

				if err != nil {
					batchProcessingError = err
					break workflowrun
				}

				if len(nextTaskRunBatch.TaskRunIDs) == 0 {
					break workflowrun
				}
				distinctTaskRunIDs := wfRun.toDistinctIDs(nextTaskRunBatch.TaskRunIDs)
				nextTaskRunBatch.TaskRunIDs = distinctTaskRunIDs
				batchChannel, errChannel = wfRun.tbSupervisor.Supervise(nextTaskRunBatch, handlerMap)
			}
		case err = <-errChannel:
			{
				batchProcessingError = errors.New(fmt.Sprintf("Received failure from taskBatchSupervisor %v", err))
				break workflowrun
			}
		}
	}
	if batchProcessingError != nil {
		return wfRun.failWorkflowRun(inprogressWfRun, batchProcessingError)
	}

	return wfRun.concludeWorkflowRun(inprogressWfRun)
}

func (wfRun wfRunSupervisor) failWorkflowRun(run models.WorkflowRun, err error) error {
	fmt.Printf("Failed WorkflowRun due to %v\n", err)
	run.Lifecycle = models.FAILED
	_, err = wfRun.wfRunStore.Update(run)
	return err
}

func (wfRun wfRunSupervisor) concludeWorkflowRun(run models.WorkflowRun) error {
	run.Lifecycle = models.DONE
	_, err := wfRun.wfRunStore.Update(run)
	return err
}

func (wfRun wfRunSupervisor) getNextTaskRunBatch(batch tbsupervisor.TaskBatch) (tbsupervisor.TaskBatch, error) {
	nextContext := batch.Context
	nextRunIDS := batch.NextTaskRunIDs
	return tbsupervisor.TaskBatch{
		TaskRunIDs: nextRunIDS,
		Context:    nextContext,
	}, nil
}

func (wfRun wfRunSupervisor) toDistinctIDs(ids []string) []string {
	m := make(map[string]bool)
	var deDuped []string
	for _, id := range ids {
		if _, ok := m[id]; !ok {
			m[id] = true
			deDuped = append(deDuped, id)
		}
	}
	return deDuped
}
