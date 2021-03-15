# Flow #

## Description ##
Flow is the library to run and manage async workflows

## Responsibilities ##

- Triggers workflow runs in an idempotent fashion.
- Ensures partial failures do not compromise the integrity of the system.
- Ensures retries are performed without adverse side-effects.
- Supports concurrent executions of the workflow.
- Enables execution of a flow described as a JSON.
- Enables execution of arbitrary Handlers that executes the discrete steps of the flow.
- Performs status checks of the overarching flow as well as the discrete tasks.
- Performs retries and error handling strategies as described by the flow definition.
- Can be included as a drop in library to the existing services.
- Will initialize its own dependencies upon startup.

## API ##

StartRun(flow Flow, context FlowContext, handlerMap *HandlerMap) (RunID, error)

Input
- Flow: The graph of steps to be executed by the runner.
- FlowContext:  The map of input context needed by the flow to execute.
- HandlerMap: The mapping of the step name to the handler associated with it.

Output
- RunID: The id of the running flow  after a successful acceptance by the runner.
- Error: Error indicating any issues accepting the flow to be run.

## Overarching algorithm ##
The runner has a sync path and an async path.

Sync path:
- Generate RunID
- Generate Task Graph from the flow definition.
- Validate the Task Graph.
- Persist the TaskGraph in the underlying store.
- Trigger the run by passing the runID, context and handlerMap to the trigger.

Async path:
The flow trigger orchestrates the async flow by coordinating the interaction between the following three components:
#### Supervisor ####  
- Polls the WorkflowRun table to check whether the flow was committed as indicated by the RunID
- Checks if the WorkflowRun is in SUBMITTED state.
- Flips the WorkflowRun state to INPROGRESS and persists the payload in the WorkflowRun table.
- Submits the initial TaskRuns along with the stored context to the TaskRun supervisor to be executed.
- Waits for completion or errors from the batch from the TaskRun supervisor. 
    - Upon a successful completion, retrieves the next set of tasks from the TaskRun table.
    - Upon an error, logs and mark the Run as FAILED. 
- If there are no next set of tasks, mark the Run as DONE. and exit out of the supervisor.
#### TaskBatchSupervisor ####
- Receives task batch ids from the Supervisor.
- Receives the context from the Supervisor.
- Spawns the requisite TaskRunner routines by passing it the TaskID and the context.
- Waits for either an error or a completion context from the TaskRunner.
    - When it receives a CompletionContext from the taskRunner, it is merged to form a StepContext 
      which is sent back to the Supervisor.
    - If an error is received, the runContext is shutdown for all the TaskRunners and 
      the error is sent back to the supervisor.
 - Once there are no more Tasks, the merged completion context is sent back to the 
   supervisor, and the TaskBatchSupervisor is shutdown.

#### TaskRunner ####

- Receives a TaskRunID and context from the TaskBatchSupervisor
- Retrieves the TaskRun from the TaskRun table and checks for SUBMITTED state.
- Flips the state of the TaskRun to InProgress and persists the state.
- Retrieves the handler for the stepName from the HandlerMap.
- Executes the handler by passing the context to it.
- Waits for the handler to be executed by checking the status.
- Upon encountering an error while executing the handler, the task status will
  be set to 'FAILED', and the error will be sent up to the TaskBatchSupervisor.
- Upon receiving a 'DONE' or 'FAILED' from the status check
  - If 'FAILED', the taskun state will be set to 'FAILED', and an error will be sent to the TaskBatchSupervisor
  - If 'DONE', the taskrun state will be set to 'DONE', and the StepContext will be sent back to the TaskBatchSupervisor         


#### Generator ####

Given a json flow definition as follows. The definition for the workflow should consist of the following attributes:	

First things first, lets see how can a struct be generated dynamically.	

-> Shape of the context	
-> Shape of the request	
-> Shape of the response	
-> Shape of the API	
-> The steps to be executed as part of the workflow	

The generator will create the following artifacts:	
1. The struct for the context.	
2. The struct for the request.	
3. The struct for the response.	
4. The interface for the API which will include the structs as part of its signature.	
5. The workflow struct that encapsulates the steps. The step interface is the property of the workflow runner.	
6. Handler interfaces for the steps. These need to be implemented by the user and injected.	
7. An initial implementation of the API that 	

{ "context": {	
    "unitname":"string",	
    "accountid":"string"     	
  },	
  "flow": "createunit",	
  "steps": [	
      {"name":"createnamespace",	
      "isstart":true,	
      "next":[]	
      }	
  ]	
}	

This should generate a couple of artifacts under cloudblox.generator/createunit	
- service.go	
  This will contain the necessary service interfaces. In the case of CreateUnit it will be:	

  type CreateUnitService interface {	
    StartCreateUnit(StartCreateUnitRequest) StartCreateUnitResponse	
    GetCreateUnit(GetCreateUnitRequest) GetCreateUnitResponse  	
  }	

  Some of the struct definitions will be defined as well:	
  type struct CreateUnitContext {	
    UnitName string,	
    AccountID string,	
  }	

  type StartCreateUnitRequest struct {	
    Context CreateUnitContext	
  }	

  type StartCreateUnitResponse struct {	
    RunID string,	
  }	

  type GetUnitRequest struct {	
     RunID string,	
  }	

  type GetUnitResponse struct {	
    Context CreateUnitContext,	
    StepName string,	
    Status string,	
  }	

- createunitworkflow.go	
type CreateUnitWorkflow struct {	
    start []Steps,	
}	

- createnamespacestep.go. 	
This will contain the struct which will implement the step interface as follows:	
type Step interface {	
      Name() string	
	  IsStart() bool	
	  Next() []Step	
}	

type NewCreateNamespaceStep(handler CreateNamespaceHandler) Step {	
    return &createNamespaceStep{	
       handler: handler, 	
    }	
}	

type createNamespaceStep struct {	
      handler CreateNamespaceHandler,	
  }	

func (cn *createNamespaceStep) Name() string {	
    return "createnamespace"	
}	

func (cn *createNamespaceStep) IsStart() bool {	
    return true	
}	

func (cn *createNamespaceStep Next() []Step {	
    return []Step{}	
}	

type CreateNamespaceHandler interface {	
    Execute(CreateUnitContext) (CreateUnitContext, error)	
    Status(CreateUnitContext) (CreateUnitContext, error)	
}     