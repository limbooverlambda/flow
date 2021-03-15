package storage

import (

	"errors"
	"fmt"
	"log"

	"flow/db"
	"flow/models"
)

//ErrEntityExists is thrown by the storage layer if the entity exists
var ErrEntityExists = errors.New("entity exists")

var ErrEntityDoesNotExist = errors.New("entity doesn't exit")

var ErrorDuplicateTaskRun = errors.New("duplicate taskrun found")

type TaskRunStore interface {
	Store(taskRun models.TaskRun) error
	Get(taskRunID string) (models.TaskRun, error)
	Update(taskRun models.TaskRun) (models.TaskRun, error)
}

func NewTaskRunStore(dbCommon *db.Common) TaskRunStore {
	return &trStore{dbCommon: dbCommon}
}

func GetTaskRunSchema() string {
	return `
	  CREATE TABLE taskrun (
		id text UNIQUE,
		workflowrunid text,
		taskname text,
		lifecycle text,
		context bytea,
		nexttaskruns bytea,
		versionid bigint,
		createdat bigint,
		updatedat bigint
	  );`
}

func GetWorkflowRunSchema() string {
	return `
	  CREATE TABLE workflowrun (
		id text UNIQUE,
		lifecycle text,
		context bytea,
		initialtaskruns bytea,
		versionid bigint,
		createdat bigint,
		updatedat bigint
	  );
	`
}


type WorkflowRunStore interface {
	Get(wfRunID string) (models.WorkflowRun, error)
	Store(workflowRun models.WorkflowRun) error
	Update(workflowRun models.WorkflowRun) (models.WorkflowRun, error)
}

func NewWorkflowRunStore(dbCommon *db.Common) WorkflowRunStore {
	return &wfRunStore{dbCommon: dbCommon}
}


type wfRunStore struct {
	dbCommon *db.Common
}

func (w *wfRunStore) Get(wfRunID string) (models.WorkflowRun, error) {
	query := models.WorkflowRunQuery{
		RunID: wfRunID,
	}
	pProc := db.PreProcessFuncs{}
	pProc.ConvertEntityFn = func(entity interface{}) (interface{}, error) {
		q, ok := entity.(models.WorkflowRunQuery)
		if !ok {
			err := fmt.Errorf("unable to convert to Query: %v", query)
			return models.WorkflowRunQuery{}, err
		}
		return q, nil
	}
	pProc.ValidateEntityFn = func(entity interface{}) (interface{}, error) {
		q := entity.(models.WorkflowRunQuery)
		if q.RunID == "" {
			return models.WorkflowRunQuery{}, errors.New("workflow runid missing")
		}
		return q, nil
	}
	pProc.CheckForEmptinessFn = func(entity interface{}) error {
		q := entity.(models.WorkflowRunQuery)
		emptyQuery := models.WorkflowRunQuery{}
		if q == emptyQuery {
			return fmt.Errorf("query cannot be empty")
		}
		return nil
	}
	var results []models.WorkflowRun
	err := w.dbCommon.Get(w.tablename(), query, &results, pProc)
	if err != nil {
		return models.WorkflowRun{}, err
	}
	if len(results) == 0 {
		return models.WorkflowRun{}, ErrEntityDoesNotExist
	}
	return results[0], nil
}

func (w *wfRunStore) Store(workflowRun models.WorkflowRun) error {
	pProc := db.PreProcessFuncs{}
	pProc.ConvertEntityFn = func(entity interface{}) (interface{}, error) {
		wr, ok := entity.(models.WorkflowRun)
		if !ok {
			err := fmt.Errorf("unable to convert to taskRun: %v", entity)
			return models.WorkflowRun{}, err
		}
		return wr, nil
	}
	//ValidateEntityFn is used to check whether the payload for the store is valid
	pProc.ValidateEntityFn = func(entity interface{}) (interface{}, error) {
		//Default operation
		return entity, nil
	}
	pProc.GetErrorMessageFn = func(entity interface{}, errorName string) error {
		if errorName == "integrity_constraint_violation" {
			return ErrEntityExists
		}
		return fmt.Errorf("WorkflowRun store failed due to %v", errorName)
	}
	return w.dbCommon.Store(w.tablename(), workflowRun, pProc)
}

func (w *wfRunStore) Update(workflowRun models.WorkflowRun) (models.WorkflowRun, error) {
	pProc := db.PreProcessFuncs{}
	pProc.ConvertEntityFn = func(entity interface{}) (interface{}, error) {
		wr, ok := entity.(models.WorkflowRun)
		if !ok {
			err := fmt.Errorf("unable to convert to workflowRun: %v", entity)
			return models.WorkflowRun{}, err
		}
		return wr, nil
	}
	vid, err := w.dbCommon.Update(w.tablename(), workflowRun, pProc)
	if err != nil {
		log.Printf("Failed to update due to %v\n", err)
		return models.WorkflowRun{}, err
	}
	workflowRun.VersionID = *vid
	return workflowRun, nil
}

func (w *wfRunStore) tablename() string {
	return "WorkflowRun"
}

type trStore struct {
	dbCommon *db.Common
}

//Store will store the TaskRun entity to the underlying storage
//If the storage fails for any reason the error will be surfaced back to the caller
func (trStore *trStore) Store(tr models.TaskRun) (err error) {
	pProc := db.PreProcessFuncs{}
	pProc.ConvertEntityFn = func(entity interface{}) (interface{}, error) {
		wr, ok := entity.(models.TaskRun)
		if !ok {
			err = fmt.Errorf("unable to convert to taskRun: %v", entity)
			return models.TaskRun{}, err
		}
		return wr, nil
	}
	//ValidateEntityFn is used to check whether the payload for the store is valid
	pProc.ValidateEntityFn = func(entity interface{}) (interface{}, error) {
		//Default operation
		return entity, nil
	}
	pProc.GetErrorMessageFn = func(entity interface{}, errorName string) error {
		if errorName == "integrity_constraint_violation" {
			tr := entity.(models.TaskRun)
			log.Printf("TaskRunID %v exists", tr.TaskRunID)
			return ErrorDuplicateTaskRun
		}
		return fmt.Errorf("%v", "TaskRun store failed")
	}
	return trStore.dbCommon.Store(trStore.tablename(), tr, pProc)
}

func (trStore *trStore) Get(taskRunID string) (models.TaskRun, error) {
	query := models.TaskRunQuery{
		TaskRunID: taskRunID,
	}
	pProc := db.PreProcessFuncs{}
	pProc.ConvertEntityFn = func(entity interface{}) (interface{}, error) {
		q, ok := entity.(models.TaskRunQuery)
		if !ok {
			err := fmt.Errorf("unable to convert to Query: %v", query)
			return models.TaskRunQuery{}, err
		}
		return q, nil
	}
	pProc.ValidateEntityFn = func(entity interface{}) (interface{}, error) {
		q := entity.(models.TaskRunQuery)
		if q.TaskRunID == "" {
			return models.TaskRunQuery{}, errors.New("task runid missing")
		}
		return q, nil
	}
	pProc.CheckForEmptinessFn = func(entity interface{}) error {
		q := entity.(models.TaskRunQuery)
		emptyQuery := models.TaskRunQuery{}
		if q == emptyQuery {
			return fmt.Errorf("query cannot be empty")
		}
		return nil
	}
	var results []models.TaskRun
	err := trStore.dbCommon.Get(trStore.tablename(), query, &results, pProc)
	if err != nil {
		return models.TaskRun{}, err
	}
	if len(results) == 0 {
		return models.TaskRun{}, ErrEntityDoesNotExist
	}
	return results[0], nil
}

func (trStore *trStore) Update(taskRun models.TaskRun) (models.TaskRun, error) {
	pProc := db.PreProcessFuncs{}
	pProc.ConvertEntityFn = func(entity interface{}) (interface{}, error) {
		tr, ok := entity.(models.TaskRun)
		if !ok {
			err := fmt.Errorf("unable to convert to taskRun: %v", entity)
			return models.TaskRun{}, err
		}
		return tr, nil
	}
	vid, err := trStore.dbCommon.Update(trStore.tablename(), taskRun, pProc)
	if err != nil {
		return models.TaskRun{}, err
	}
	taskRun.VersionID = *vid
	return taskRun, nil
}

func (trStore *trStore) tablename() string {
	return "taskrun"
}
