package trun

import (
	"fmt"
	"github.com/stretchr/testify/mock"
	"testing"

	"flow/models"
	"flow/steps"
	"flow/storage"
)

func Test_taskrunner_Run(t *testing.T) {

	var fakeTaskRunStore storage.FakeTaskRunStore
	fakeTaskRunStore.On("Get", mock.Anything).Return(models.TaskRun{
		TaskRunID:      "",
		WorkflowRunID:  "",
		TaskName:       "",
		NextTaskRunIDS: nil,
		Status:         models.SUBMITTED,
		Context:        nil,
		VersionID:      0,
		CreatedAt:      0,
		UpdatedAt:      0,
	}, nil)

	fakeTaskRunStore.On("Update", mock.Anything).Return(models.TaskRun{
		TaskRunID:      "",
		WorkflowRunID:  "",
		TaskName:       "",
		NextTaskRunIDS: nil,
		Status:         models.DONE,
		Context:        nil,
		VersionID:      0,
		CreatedAt:      0,
		UpdatedAt:      0,
	}, nil)

	type fields struct {
		taskRunStore storage.TaskRunStore
	}
	type args struct {
		taskRunID   string
		flowContext models.FlowContext
		handlerMap  *steps.HandlerMap
	}
	tests := []struct {
		name   string
		fields fields
		args   args
		want   TaskRunSignals
	}{
		{
			name: "Happy Path",
			fields: fields{
				taskRunStore: &fakeTaskRunStore,
			},
			args: args{
				taskRunID:   "runID",
				flowContext: models.FlowContext{},
				handlerMap:  getFakeHandlerMap(),
			},
			want: TaskRunSignals{
				TaskRunStatusChan: nil,
				TaskRunErrorChan:  nil,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tr := taskrunner{
				taskRunStore: tt.fields.taskRunStore,
			}
			actualTaskRunSignals := tr.Run(tt.args.taskRunID, tt.args.flowContext, tt.args.handlerMap)
			errChan := actualTaskRunSignals.TaskRunErrorChan
			statusChan := actualTaskRunSignals.TaskRunStatusChan
			var err error
			var status TaskRunStatus
		loop:
			for {
				select {
				case status = <-statusChan:
					break loop
				case err = <-errChan:
					break loop
				}
			}
			fmt.Printf("Status is %v\n", status)
			fmt.Printf("Err is %v\n", err)
		})
	}
}

func getFakeHandlerMap() *steps.HandlerMap {
	return &steps.HandlerMap{
		"step1": step1Handler{},
	}
}

type step1Handler struct{}

func (step1Handler) Name() string {
	return "step1"
}

func (step1Handler) Run(flowContext models.FlowContext) (models.FlowContext, error) {
	return flowContext, nil
}

func (step1Handler) Status(flowContext models.FlowContext) (steps.StepStatus, error) {
	return steps.StepDone, nil
}
