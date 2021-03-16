package workflow

import (
	"github.com/stretchr/testify/mock"
	"testing"

	"flow/models"
	"flow/steps"
	"flow/storage"
)

func TestToFlowGraph(t *testing.T) {
	s3 := step{
		name:     "s3",
		nextstep: []steps.Step{},
	}
	s2 := step{
		name:     "s2",
		nextstep: []steps.Step{s3},
	}
	s1 := step{
		name:     "s1",
		nextstep: []steps.Step{s2},
	}

	f := flow{
		name:       "foo",
		startsteps: []steps.Step{s1},
	}
	var fakeWfRunStore storage.FakeWorkflowRunStore
	var fakeTRunStore storage.FakeTaskRunStore

	fakeTRunStore.On("Store", mock.Anything).Return(nil)
	fakeWfRunStore.On("Store", mock.Anything).Return(nil)

	fr := fUnwrap{
		trStore: &fakeTRunStore,
		wrStore: &fakeWfRunStore,
	}

	_, _ = fr.ToFlowGraph("runID", models.FlowContext{}, f)
	_ = models.Flowgraph{
		Flowsteps: []models.FlowStep{
			models.FlowStep{
				ParallelTaskRuns: []models.TaskRun{
					models.TaskRun{
						TaskName:       "s1",
						TaskRunID:      "TR-s1-runID",
						NextTaskRunIDS: []string{"TR-s2-runID"},
					},
				},
			},
			{ParallelTaskRuns: []models.TaskRun{
				{TaskName: "s2",
					TaskRunID:      "TR-s2-runID",
					NextTaskRunIDS: []string{"TR-s3-runID"}},
			}},
			{ParallelTaskRuns: []models.TaskRun{
				models.TaskRun{
					TaskName:  "s3",
					TaskRunID: "TR-s3-runID",
				},
			}},
		},
	}
	//TODO: Fix the assert
	//assert.Equal(t, fg, efg)
}
