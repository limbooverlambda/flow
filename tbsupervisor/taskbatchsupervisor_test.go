package tbsupervisor

import (
	"errors"
	"github.com/stretchr/testify/assert"
	"reflect"
	"testing"

	"flow/models"
	"flow/steps"
	"flow/trun"
)

func Test_taskBatchSupervisor_Supervise(t *testing.T) {
	type fields struct {
		taskRunner trun.TaskRunner
	}
	type args struct {
		batch      TaskBatch
		handlerMap *steps.HandlerMap
	}
	type expectations struct {
		expectedTaskRunIDs []string
		expectedContext    models.FlowContext
		expectedError      error
	}
	var fTaskRunner fakeTaskRunner
	tests := []struct {
		name         string
		fields       fields
		args         args
		expectations expectations
	}{
		{
			name: "Happy Path",
			fields: fields{
				taskRunner: fTaskRunner,
			},
			args: args{
				batch: TaskBatch{
					TaskRunIDs: []string{"a", "b", "c"},
					Context:    map[string]interface{}{},
				},
				handlerMap: nil,
			},
			expectations: expectations{
				expectedTaskRunIDs: []string{"a", "b", "c"},
				expectedContext:    models.FlowContext{"a": "done", "b": "done", "c": "done"},
				expectedError:      nil,
			},
		},
		{
			name: "One of the tasks failing",
			fields: fields{
				taskRunner: fTaskRunner,
			},
			args: args{
				batch: TaskBatch{
					TaskRunIDs: []string{"a", "b", "fail"},
					Context:    map[string]interface{}{},
				},
				handlerMap: nil,
			},
			expectations: expectations{
				expectedTaskRunIDs: nil,
				expectedContext:    nil,
				expectedError:      errors.New("failed TaskRun"),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tbs := taskBatchSupervisor{
				taskRunner: tt.fields.taskRunner,
			}
			tbChan, errChan := tbs.Supervise(tt.args.batch, tt.args.handlerMap)
			var tb TaskBatch
			var tbError error
		testloop: //label is needed because you will be breaking out of the innermost select
			for {
				select {
				case tb = <-tbChan:
					break testloop
				case tbError = <-errChan:
					break testloop
				}
			}
			assert.Equal(t, tb.TaskRunIDs, tt.expectations.expectedTaskRunIDs)
			assert.Equal(t, tbError, tt.expectations.expectedError)
			if !reflect.DeepEqual(tb.Context, tt.expectations.expectedContext) {
				assert.Fail(t, "Incorrect context")
			}

		})
	}
}

type fakeTaskRunner struct{}

func (fakeTaskRunner) Run(taskRunID string, flowContext models.FlowContext, handlerMap *steps.HandlerMap) trun.TaskRunSignals {
	tRunChannel := make(chan trun.TaskRunStatus)
	tErrorChannel := make(chan error)
	go func() {
		defer close(tRunChannel)
		defer close(tErrorChannel)
		if taskRunID == "fail" {
			tRunChannel <- trun.TaskRunStatus{
				RunContext: nil,
				Status:     models.FAILED,
			}
			return
		}
		tRunChannel <- trun.TaskRunStatus{
			RunContext: map[string]interface{}{taskRunID: "done"},
			Status:     models.DONE,
		}
	}()
	return trun.TaskRunSignals{
		TaskRunStatusChan: tRunChannel,
		TaskRunErrorChan:  tErrorChannel,
	}
}
