package models

import (
	"bytes"
	"database/sql/driver"
	"encoding/gob"
	"errors"
)

//FlowContext contains the attributes that's needed for executing the step
type FlowContext map[string]interface{}

func (fc FlowContext) Value() (driver.Value, error) {
	buf := bytes.Buffer{}
	enc := gob.NewEncoder(&buf)
	err := enc.Encode(&fc)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (fc *FlowContext) Scan(src interface{}) error {
	if src == nil {
		return nil
	}
	blob, ok := src.([]byte)
	if !ok {
		return errors.New("type assertion .([]byte) failed")
	}
	err := gob.NewDecoder(bytes.NewReader(blob)).Decode(fc)
	if err != nil {
		return err
	}
	return nil
}

type TaskRunIDs []string

func (trids TaskRunIDs) Value() (driver.Value, error) {
	buf := bytes.Buffer{}
	enc := gob.NewEncoder(&buf)
	err := enc.Encode(&trids)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (trids *TaskRunIDs) Scan(src interface{}) error {
	if src == nil {
		return nil
	}
	blob, ok := src.([]byte)
	if !ok {
		return errors.New("type assertion .([]byte) failed")
	}
	err := gob.NewDecoder(bytes.NewReader(blob)).Decode(trids)
	if err != nil {
		return err
	}
	return nil
}

//RunID is the id of the flow run
type RunID string

type Flowgraph struct {
	Flowsteps []FlowStep
}
type FlowStep struct {
	ParallelTaskRuns []TaskRun
}

//Lifecycle is the lifecycle of the TaskRun
type Lifecycle string

const (
	//INPROGRESS Task is in-progress
	INPROGRESS Lifecycle = "INPROGRESS"
	//FAILED Task has failed
	FAILED Lifecycle = "FAILED"
	//DONE Task is done
	DONE Lifecycle = "DONE"
	//SUBMITTED Task has been submitted for eventual execution
	SUBMITTED Lifecycle = "SUBMITTED"
	//CLAIMED Task has been claimed by a worker
	CLAIMED Lifecycle = "CLAIMED"
	//RUNNING
	RUNNING Lifecycle = "RUNNING"
)

//Query is the payload representing a Query object
//used to retrieve the TaskRun from the underlying store
type TaskRunQuery struct {
	TaskRunID string    `db:"id"`
	Status    Lifecycle `db:"lifecycle"`
	VersionID int64     `db:"versionid"`
}

type TaskRun struct {
	TaskRunID      string      `db:"id" key:"id"`
	WorkflowRunID  string      `db:"workflowrunid"`
	TaskName       string      `db:"taskname"`
	NextTaskRunIDS TaskRunIDs  `db:"nexttaskruns"`
	Context        FlowContext `db:"context" update:"context"`
	Status         Lifecycle   `db:"lifecycle" update:"lifecycle"`
	VersionID      int64       `db:"versionid" key:"versionid"`
	CreatedAt      int64       `db:"createdat"`
	UpdatedAt      int64       `db:"updatedat" update:"updatedat"`
}

type WorkflowRunQuery struct {
	RunID     string    `db:"id"`
	Status    Lifecycle `db:"lifecycle"`
	VersionID int64     `db:"versionid"`
}

type WorkflowRun struct {
	RunID             string      `db:"id" key:"id"`
	Context           FlowContext `db:"context" update:"context"`
	Lifecycle         Lifecycle   `db:"lifecycle" update:"lifecycle"`
	InitialTaskRunIDs TaskRunIDs  `db:"initialtaskruns"`
	VersionID         int64       `db:"versionid" key:"versionid"`
	CreatedAt         int64       `db:"createdat"`
	UpdatedAt         int64       `db:"updatedat" update:"updatedat"`
}
