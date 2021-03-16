package workflow

import (
	"errors"
	"fmt"
	"log"
	"reflect"
	"time"

	"flow/models"
	"flow/steps"
	"flow/storage"
)

//flowUnwrapper is being used for "unwrapping a workflow and persisting it"
type FlowUnwrapper interface {
	//unwraps the flowgraph and persists it
	ToFlowGraph(runID string, flowContext models.FlowContext, flow Flow) (models.Flowgraph, error)
}

func NewFlowUnwrapper(workflowRunStore storage.WorkflowRunStore, taskRunStore storage.TaskRunStore) FlowUnwrapper {
	return fUnwrap{
		trStore: taskRunStore,
		wrStore: workflowRunStore,
	}
}

type fUnwrap struct {
	trStore storage.TaskRunStore
	wrStore storage.WorkflowRunStore
}

func (fr fUnwrap) ToFlowGraph(runID string, flowContext models.FlowContext, flow Flow) (models.Flowgraph, error) {
	var flowSteps []models.FlowStep
	var buffer []interface{}
	buffer = append(buffer, flow.StartSteps())
	processFn := func(element interface{}) {
		var steps = fr.reflectToStepSlice(element)
		flowSteps = append(flowSteps, fr.extractFlowStep(runID, steps))
	}
	nextStepsFn := func(element interface{}) []interface{} {
		var ss = fr.reflectToStepSlice(element)
		var nextSteps []interface{}
		for _, s := range ss {
			for _, sNext := range s.NextSteps() {
				nextSteps = append(nextSteps, sNext)
			}
		}
		return nextSteps
	}
	bfs(buffer, processFn, nextStepsFn)
	taskGraph := models.Flowgraph{
		Flowsteps: flowSteps,
	}
	err := fr.persistFlowGraph(runID, flowContext, flow, taskGraph)
	return taskGraph, err
}

func (fr fUnwrap) persistFlowGraph(parentRunID string, flowContext models.FlowContext, flow Flow, flowGraph models.Flowgraph) error {
	for _, flowStep := range flowGraph.Flowsteps {
		for _, taskRun := range flowStep.ParallelTaskRuns {
			err := fr.persistTaskRun(taskRun)
			if err != nil && !errors.As(err, &storage.ErrEntityExists) {
				return fmt.Errorf("failed to persist task graph for flow %w", err)
			}
			if errors.Is(err, storage.ErrorDuplicateTaskRun) {
				log.Printf("TaskRunID %v has been persisted, skipping\n", taskRun.TaskRunID)
			}
		}
	}
	wfRun := fr.toNewWorkflowRun(parentRunID, flowContext, flow)
	err := fr.persistWorkflowRun(wfRun)
	if err != nil {
		return fmt.Errorf("failed to persist workflowRun for flow %w", err)
	}
	return err
}

func (fr fUnwrap) persistTaskRun(taskRun models.TaskRun) error {
	return fr.trStore.Store(taskRun)
}

func (fr fUnwrap) persistWorkflowRun(workflowRun models.WorkflowRun) error {
	return fr.wrStore.Store(workflowRun)
}

func (fr fUnwrap) reflectToStepSlice(element interface{}) []steps.Step {
	var stps []steps.Step
	s := reflect.ValueOf(element)
	if s.Kind() != reflect.Slice {
		//panic("InterfaceSlice() given a non-slice type")
		step := s.Interface().(steps.Step)
		stps = append(stps, step)
		return stps
	}
	for i := 0; i < s.Len(); i++ {
		step := s.Index(i).Interface().(steps.Step)
		stps = append(stps, step)
	}
	return stps
}

func (fr fUnwrap) extractFlowStep(runID string, steps []steps.Step) models.FlowStep {
	var parallelTaskRuns []models.TaskRun
	for _, step := range steps {
		tr := fr.toNewTaskRun(runID, step)
		parallelTaskRuns = append(parallelTaskRuns, tr)
	}
	return models.FlowStep{
		ParallelTaskRuns: parallelTaskRuns,
	}
}

func (fr fUnwrap) toNewTaskRun(runID string, step steps.Step) models.TaskRun {
	timestamp := time.Now().Unix()
	taskRunID := fr.toTaskRunID(runID, step.Name())
	var nextTaskRunIDS []string
	for _, nid := range step.NextSteps() {
		nextTaskRunID := fr.toTaskRunID(runID, nid.Name())
		nextTaskRunIDS = append(nextTaskRunIDS, nextTaskRunID)
	}
	return models.TaskRun{
		TaskRunID:      taskRunID,
		WorkflowRunID:  runID,
		TaskName:       step.Name(),
		NextTaskRunIDS: nextTaskRunIDS,
		Status:         models.SUBMITTED,
		VersionID:      1,
		CreatedAt:      timestamp,
		UpdatedAt:      timestamp,
	}
}

func (fr fUnwrap) toNewWorkflowRun(parentRunID string, workflowContext models.FlowContext, flow Flow) models.WorkflowRun {
	timestamp := time.Now().Unix()
	return models.WorkflowRun{
		RunID:             parentRunID,
		Context:           workflowContext,
		Lifecycle:         models.SUBMITTED,
		InitialTaskRunIDs: fr.toInitialTaskRunIDs(parentRunID, flow),
		VersionID:         1,
		CreatedAt:         timestamp,
		UpdatedAt:         timestamp,
	}
}

func (fr fUnwrap) toInitialTaskRunIDs(parentRunID string, flow Flow) []string {
	var initTaskRunIDs []string
	for _, s := range flow.StartSteps() {
		trID := fr.toTaskRunID(parentRunID, s.Name())
		initTaskRunIDs = append(initTaskRunIDs, trID)
	}
	return initTaskRunIDs
}

func (fr fUnwrap) toTaskRunID(parentRunID string, taskName string) string {
	return fmt.Sprintf("TR-%v-%v", taskName, parentRunID)
}
