package poll

import (
	"errors"
	"log"
	"time"

	"flow/models"
	"flow/storage"
)

//TODO: Add the appropriate time updates decorating the payload

type WfRunPoller interface {
	Poll(runID string) (models.WorkflowRun, error)
}

func NewWorkflowRunPoller(workflowRunStore storage.WorkflowRunStore) WfRunPoller {
	return wfRunPoller{
		wfRunStore: workflowRunStore,
		ticker:     time.NewTicker(1 * time.Second),
	}
}

type wfRunPoller struct {
	wfRunStore storage.WorkflowRunStore
	ticker     *time.Ticker
}

func (p wfRunPoller) Poll(runID string) (models.WorkflowRun, error) {
	defer p.ticker.Stop()
	var wfRun models.WorkflowRun
	var pollError error
loop:
	for {
		select {
		case <-p.ticker.C:
			{
				wfRun, pollError = p.checkForWfRun(runID)
				if pollError != nil  {
					log.Printf("Issue polling for wfRun %v\n", pollError)
					break loop
				}
				if wfRun.Lifecycle != models.SUBMITTED {
					pollError = errors.New("workflow run not in acceptable state")
					break loop
				}
				if wfRun.Lifecycle == models.SUBMITTED {
					break loop
				}
			}
		}
	}
	return wfRun, pollError
}

func (p wfRunPoller) checkForWfRun(id string) (models.WorkflowRun, error) {
	return p.wfRunStore.Get(id)
}
