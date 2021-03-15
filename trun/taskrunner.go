package trun

import (

	"errors"
	"fmt"

	"flow/models"
	"flow/steps"
	"flow/storage"
)

type TaskRunStatus struct {
	RunContext     models.FlowContext
	Status         models.Lifecycle
	NextTaskRunIDs []string
}

type TaskRunSignals struct {
	TaskRunStatusChan <-chan TaskRunStatus
	TaskRunErrorChan  <-chan error
}

type TaskRunner interface {
	Run(taskRunID string, flowContext models.FlowContext, handlerMap *steps.HandlerMap) TaskRunSignals
}

func NewTaskRunner(taskRunnerStore storage.TaskRunStore) TaskRunner {
	return taskrunner{taskRunStore: taskRunnerStore}
}

type taskrunner struct {
	taskRunStore storage.TaskRunStore
}

func (tr taskrunner) Run(taskRunID string, flowContext models.FlowContext, handlerMap *steps.HandlerMap) TaskRunSignals {
	taskRunErrorChan := make(chan error)
	taskRunStatusChan := make(chan TaskRunStatus)
	go func() {
		defer close(taskRunErrorChan)
		defer close(taskRunStatusChan)
		//Retrieve the TaskRun
		taskRun, err := tr.taskRunStore.Get(taskRunID)
		if err != nil {
			taskRunErrorChan <- errors.New("unable to retrieve taskrun")
			return
		}

		for {
			taskRun, err = tr.process(taskRun, flowContext, handlerMap)
			if err != nil {
				taskRunErrorChan <- fmt.Errorf("error encountered while processing taskrun %v: %v",taskRun.TaskName, err)
				return
			}
			if tr.inTerminalState(taskRun) {
				break
			}
		}
		taskRunStatus := TaskRunStatus{
			NextTaskRunIDs: taskRun.NextTaskRunIDS,
			RunContext:     taskRun.Context,
			Status:         models.Lifecycle(taskRun.Status),
		}
		taskRunStatusChan <- taskRunStatus
		return
	}()
	return TaskRunSignals{
		TaskRunStatusChan: taskRunStatusChan,
		TaskRunErrorChan:  taskRunErrorChan,
	}
}

func (tr taskrunner) inTerminalState(run models.TaskRun) bool {
	lifecycle := models.Lifecycle(run.Status)
	return lifecycle == models.FAILED || lifecycle == models.DONE
}

func (tr taskrunner) process(run models.TaskRun, context models.FlowContext, handlerMap *steps.HandlerMap) (models.TaskRun, error) {
	state := models.Lifecycle(run.Status)
	var taskRun models.TaskRun
	var trErr error
	switch state {
	case models.DONE:
		taskRun, trErr = tr.processDoneTask(run)
	case models.FAILED:
		taskRun, trErr = tr.processFailedTask(run)
	case models.CLAIMED:
		taskRun, trErr = tr.processClaimedTask(run)
	case models.SUBMITTED:
		taskRun, trErr = tr.processSubmittedTask(run)
	case models.INPROGRESS:
		taskRun, trErr = tr.processInProgressTask(run, context, handlerMap)
	case models.RUNNING:
		taskRun, trErr = tr.processRunningTask(run, handlerMap)
	}
	return taskRun, trErr
}

func (tr taskrunner) processDoneTask(run models.TaskRun) (models.TaskRun, error) {
	//Store the done task
	return tr.taskRunStore.Update(run)
}

func (tr taskrunner) processFailedTask(run models.TaskRun) (models.TaskRun, error) {
	//Store the failed task
	return tr.taskRunStore.Update(run)
}

func (tr taskrunner) processClaimedTask(run models.TaskRun) (models.TaskRun, error) {
	run.Status = models.INPROGRESS
	return tr.taskRunStore.Update(run)
}

func (tr taskrunner) processRunningTask(run models.TaskRun, handlerMap *steps.HandlerMap) (models.TaskRun, error) {
	handler, ok := (*handlerMap)[run.TaskName]
	if !ok {
		return run, fmt.Errorf("unable to retrieve handler for task %v", run.TaskName)
	}
	status, err := handler.Status(run.Context)
	if err != nil {
		return models.TaskRun{}, err
	}
	switch status {
	case steps.StepInProgress:
		{
			run.Status = models.RUNNING
		}
	case steps.StepFailed:
		{
			run.Status = models.FAILED
		}
	case steps.StepDone:
		{
			run.Status = models.DONE
		}
	case steps.StepStatusUnknown:
		{
			run.Status = models.RUNNING
		}
	}
	return tr.taskRunStore.Update(run)
}

func (tr taskrunner) processSubmittedTask(run models.TaskRun) (models.TaskRun, error) {
	run.Status = models.CLAIMED
	return tr.taskRunStore.Update(run)
}

func (tr taskrunner) processInProgressTask(run models.TaskRun, ctx models.FlowContext, handlerMap *steps.HandlerMap) (models.TaskRun, error) {
	handler, ok := (*handlerMap)[run.TaskName]
	if !ok {
		return run, fmt.Errorf("unable to retrieve handler for task %v", run.TaskName)
	}
	nCtx, err := handler.Run(ctx)
	if err != nil {
		return run, err
	}
	run.Context = nCtx
	run.Status = models.RUNNING
	return tr.taskRunStore.Update(run)
}
