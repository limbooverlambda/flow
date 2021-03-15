package trigger

import (

	"github.com/stretchr/testify/mock"
	"testing"

	"flow/models"
	"flow/poll"
	"flow/steps"
	"flow/storage"
	"flow/tbsupervisor"
)


func Test_wfRunSupervisor_supervisorFn(t *testing.T) {
	var fakePoller poll.FakeWfRunPoller

	var fakeWfRunStore storage.FakeWorkflowRunStore

	var fakeTbSupervisor faketbsuper

	var fakeTRunStore storage.FakeTaskRunStore

	type fields struct {
		poller       poll.WfRunPoller
		wfRunStore   storage.WorkflowRunStore
		tRunStore storage.TaskRunStore
		tbSupervisor tbsupervisor.TaskBatchSupervisor
	}
	type args struct {
		runID      string
		handlerMap *steps.HandlerMap
	}
	tests := []struct {
		name   string
		fields fields
		args   args
	}{{
		name:   "happypath",
		fields: fields{
			poller:      &fakePoller,
			wfRunStore:   &fakeWfRunStore,
			tRunStore: &fakeTRunStore,
			tbSupervisor: fakeTbSupervisor,
		},
		args:   args{
			runID:      "",
			handlerMap: nil,
		},
	},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			fakePoller.On("Poll", mock.Anything).Return(models.WorkflowRun{
				Context:           nil,
				Lifecycle:         models.SUBMITTED,
				InitialTaskRunIDs: nil,
			}, nil)

			fakeWfRunStore.On("Update", mock.Anything).
				Return(models.WorkflowRun{
				Lifecycle: models.DONE,
			}, nil)

			fakeTRunStore.On("GetNextTaskRunIDs", mock.Anything).
				Return([]string{}, nil)


			wfRun := wfRunSupervisor{
				poller:       tt.fields.poller,
				wfRunStore:   tt.fields.wfRunStore,
				tRunStore: tt.fields.tRunStore,
				tbSupervisor: tt.fields.tbSupervisor,
			}
			wfRun.supervisorFn(tt.args.runID, tt.args.handlerMap)
		})
	}
}

type faketbsuper struct{}

func (faketbsuper) Supervise(tbsupervisor.TaskBatch, *steps.HandlerMap) (<-chan tbsupervisor.TaskBatch, <-chan error) {
	fakeTbChannel := make(chan tbsupervisor.TaskBatch)
	fakeErrorChannel := make(chan error)
	go func() {
		defer close(fakeTbChannel)
		defer close(fakeErrorChannel)
		fakeTbChannel <- tbsupervisor.TaskBatch{
			TaskRunIDs: nil,
			Context:    nil,
		}
	}()
	return fakeTbChannel, fakeErrorChannel
}

