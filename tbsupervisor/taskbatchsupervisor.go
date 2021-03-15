package tbsupervisor

import (

	"errors"

	"flow/models"
	"flow/steps"
	"flow/storage"
	"flow/trun"
)

type TaskBatch struct {
	TaskRunIDs []string
	NextTaskRunIDs[] string
	Context    models.FlowContext
}

type TaskBatchSupervisor interface {
	Supervise(batch TaskBatch, handlerMap *steps.HandlerMap) (<-chan TaskBatch, <-chan error)
}

func NewTaskBatchSupervisor(taskRunnerStore storage.TaskRunStore) TaskBatchSupervisor {
	return taskBatchSupervisor{taskRunner: trun.NewTaskRunner(taskRunnerStore)}
}

type taskBatchSupervisor struct {
	taskRunner trun.TaskRunner
}

func (tbs taskBatchSupervisor) Supervise(batch TaskBatch, handlerMap *steps.HandlerMap) (<- chan TaskBatch, <-chan error) {
	taskBatchErrorChan := make(chan  error)
	taskBatchChan := make(chan TaskBatch)
	go func() {
		defer close(taskBatchErrorChan)
		defer close(taskBatchChan)
		taskBatchContext := batch.Context
		taskIDsToBeRun := batch.TaskRunIDs
		var taskRunSignals []trun.TaskRunSignals
		for _, taskRunID := range taskIDsToBeRun {
			taskRunSignal := tbs.taskRunner.Run(taskRunID, taskBatchContext, handlerMap)
			taskRunSignals = append(taskRunSignals, taskRunSignal)
		}
		var batchContext models.FlowContext = make(map[string]interface{})
		var batchNextTaskRunIDS []string
		for _, taskRunSignal := range taskRunSignals {
		taskrunstatus:
			for {
				select {
				case trStatus := <-taskRunSignal.TaskRunStatusChan:
					{
						taskRunContext := trStatus.RunContext
						taskRunStatus := trStatus.Status
						if taskRunStatus == models.FAILED {
							taskBatchErrorChan <- errors.New("failed TaskRun")
							return
						}
						//Merge the returned context
						batchContext = tbs.mergeContext(batchContext, taskRunContext)
						//Merge the next TaskRunIDs
						batchNextTaskRunIDS = append(batchNextTaskRunIDS, trStatus.NextTaskRunIDs ...)
						break taskrunstatus
					}
				case trError := <-taskRunSignal.TaskRunErrorChan:
					{
						taskBatchErrorChan <- trError
						return
					}
				}
			}
		}
		aggregatedTaskBatch := TaskBatch{
			TaskRunIDs: taskIDsToBeRun,
			Context:    batchContext,
			NextTaskRunIDs: batchNextTaskRunIDS,
		}

		taskBatchChan <- aggregatedTaskBatch
		return
	}()
	return taskBatchChan, taskBatchErrorChan
}

func (tbs taskBatchSupervisor) mergeContext(context models.FlowContext, currentContext models.FlowContext) models.FlowContext {
	for k, v := range currentContext {
		context[k] = v
	}
	return context
}
