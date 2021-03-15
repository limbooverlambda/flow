package generator

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"io/ioutil"
	"os"
	"strings"
	"text/template"
)

type entityType string

const (
	stepEntity     entityType = "Step"
	workflowEntity entityType = "Flow"
)

type method struct {
	name    string
	params  []string
	returns []string
}

//FlowDefinition is ...
type FlowDefinition struct {
	Context FlowContext `json:"Context"`
	Flow    Flow        `json:"Flow"`
	Steps   []Step      `json:"Steps"`
}

type FlowContext struct {
	Name string                 `json:"Name"`
	Data map[string]interface{} `json:"Data"`
}

//Flow json ...
type Flow struct {
	Name       string   `json:"Name"`
	StartSteps []string `json:"StartSteps"`
}

func (flow *Flow) toFlowCode(imap map[string][]method) string {
	structName := fmt.Sprintf("%v", strings.ToLower(flow.Name))
	return toWorkflowEntityCode(workflowEntity, structName, flow.getValueForMethod)
}

func (flow *Flow) getValueForMethod(methodName string) string {
	switch methodName {
	case "Name":
		{
			return `"` + flow.Name + `"`
		}
	case "StartSteps":
		{
			return craftNextSteps(flow.StartSteps)
		}
	default:
		panic("bad ")
	}
}

func toReturnString(returns []string) string {
	if len(returns) == 1 {
		return returns[0]
	}
	returnStr := strings.Join(returns, ",")
	return fmt.Sprintf("(%v)", returnStr)
}

//Steps json ...
type Steps struct {
	Steps []Step `json:"Steps"`
}

func (steps *Steps) toStepsCode(imap map[string][]method) string {
	var stepCodes []string
	for _, s := range steps.Steps {
		stepCode := s.toStepCode(imap)
		stepCodes = append(stepCodes, stepCode)
	}
	return strings.Join(stepCodes, "\n")
}

//Step json ...
type Step struct {
	Name      string   `json:"Name"`
	NextSteps []string `json:"NextSteps"`
}

func (step *Step) toStepCode(imap map[string][]method) string {
	structName := fmt.Sprintf("%v", strings.ToLower(step.Name))
	return toWorkflowEntityCode(stepEntity, structName, step.getValueForMethod)
}

func toWorkflowEntityCode(et entityType,
	entityName string,
	getReturnValFn func(string) string) string {
	methods := imap[string(et)]
	entityName = strings.ToLower(entityName)
	impl := []string{fmt.Sprintf(`
	   //%v is ...
	   type %v struct{}
	`, entityName, entityName)}
	for _, m := range methods {
		mDefn := fmt.Sprintf(
			`
			//%v is ...
			func (%v) %v() %v {
			   return %v
		   }`, m.name,
			entityName,
			m.name,
			toReturnString(m.returns),
			getReturnValFn(m.name),
		)
		impl = append(impl, mDefn)
	}
	return strings.Join(impl, "\n")
}

func (step *Step) getValueForMethod(methodName string) string {
	switch methodName {
	case "Name":
		{
			return `"` + step.Name + `"`
		}
	case "NextSteps":
		{
			return craftNextSteps(step.NextSteps)
		}
	default:
		panic("bad ")
	}
}

func craftNextSteps(steps []string) string {
	var nextsteps []string
	for _, s := range steps {
		if s != "" {
			nextsteps = append(nextsteps, fmt.Sprintf("%v{}", strings.ToLower(s)))
		}
	}
	if len(nextsteps) > 0 {
		return fmt.Sprintf("[]steps.Step{%v}", strings.Join(nextsteps, ","))
	}
	return "[]steps.Step{}"
}

var imap = make(map[string][]method)

func visitor(node ast.Node) bool {
	switch t := node.(type) {
	case *ast.TypeSpec:
		{
			i, ok := t.Type.(*ast.InterfaceType)
			if ok {
				iFaceName := t.Name.Name
				//fmt.Printf("Interface: %v\n", iFaceName)
				//Initialize the codesnippet struct and generate the name for it
				var methods []method
				for _, field := range i.Methods.List {
					m := method{}
					methodName := field.Names[0].Name
					//fmt.Printf("Method name: %v\n", methodName)
					m.name = methodName
					var returns []string
					switch t := field.Type.(type) {
					case *ast.FuncType:
						{
							fieldType := t.Results.List[0]
							switch innerType := fieldType.Type.(type) {
							case *ast.Ident:
								{
									//fmt.Printf("Return type %v\n", innerType.Name)
									var name string = innerType.Name
									if name != "string" {
										name = fmt.Sprintf("workflow.%v", name)
									}
									returns = append(returns, name)
								}
							case *ast.ArrayType:
								{
									switch aitype := innerType.Elt.(type) {
									case *ast.Ident:
										{
											//fmt.Printf("Got an array with type %v\n", aitype.Name)
											var name string = aitype.Name
											if name != "string" {
												name = fmt.Sprintf("steps.%v", name)
											}
											returns = append(returns, fmt.Sprintf("[]%v", name))
										}
									}
								}
							}

						}
					}
					m.returns = returns
					methods = append(methods, m)
				}
				imap[iFaceName] = methods
			}
		}
	}
	return true
}

func Start(workflowJSONPath string) {
	//TODO: Examine the interfaces for the cloudblox.flow packages
	src := `
        package workflow

import (
	"models"
	"steps"
)

//Flow is the outline of the workflow executed by the runners
type Flow interface {
	//Name returns the name of the Workflow
	Name() string
	//StartSteps returns the initial set of steps that will trigger the flow
	StartSteps() []Step
}

//StepStatus is the status returned by the handler
//TODO: Generate the String representation for the StepStatus
type StepStatus int

const (
	//StepInProgress -> InProgress
	StepInProgress StepStatus = iota
	//StepDone -> Done
	StepDone
	//StepFailed -> Failed
	StepFailed
	//StepStatusUnknown -> Unable to retrieve Step status
	StepStatusUnknown
)

//HandlerMap is the alias from stepname to handler logic
type HandlerMap map[string]StepHandler


//Step is the discrete Step that will be executed by the runner
type Step interface {
	//Name returns the name of the Step
	Name() string
	//NextSteps returns the next set of steps that need to be executed
	NextSteps() []Step
}

//StepHandler contains the exectution and status check logic for the step
type StepHandler interface {
	//Name of the Step
	Name() string
	//Run runs the execution logic for the Step. Takes a FlowContext and decorates it with additional attributes
	Run(flowContext models.FlowContext) (models.FlowContext, error)
	//Checks the status of the step based on the current context
	Status(flowContext models.FlowContext) (StepStatus, error)
}`
	fs := token.NewFileSet()
	node, err := parser.ParseFile(fs, "", src, parser.DeclarationErrors)
	if err != nil {
		fmt.Printf("Unable to parse workflow.go due to %v\n", err)
		os.Exit(1)
	}

	ast.Inspect(node, visitor)

	var flow FlowDefinition
	flowBytes, err := ioutil.ReadFile(workflowJSONPath)
	if err != nil {
		fmt.Printf("Unable to parse workflowjson due to %v\n", err)
		os.Exit(1)
	}
	err = json.Unmarshal(flowBytes, &flow)
	if err != nil {
		fmt.Printf("Issue unmarshalling %v\n", err)
		os.Exit(1)
	}

	//fmt.Printf("Flow Definition %v\n", flow)
	packageCode := "package flow"
	importCode := `import ("models"
							"steps"
							"workflow"
							"cloudblox.db/db"
							"github.com/mitchellh/mapstructure"
                          )`
	flowCode := flow.Flow.toFlowCode(imap)
	codes := []string{packageCode, importCode, flowCode}
	for _, step := range flow.Steps {
		codes = append(codes, step.toStepCode(imap))
	}
	code := strings.Join(codes, "\n")
	b, err := format.Source([]byte(code))
	if err != nil {
		fmt.Printf("Issue formatting output %v\n", err)
		os.Exit(1)
	}

	cCode, err := templateCoder("context", contextCode, flow.Context)
	if err != nil {
		fmt.Printf("Failed to generate context code %v\n", err)
	}
	b = append(b, cCode...)

	dCode, err := templateCoder("clientcode", clientCode, flow)
	if err != nil {
		fmt.Printf("Failed to generate client code %v\n", err)
	}
	b = append(b, dCode...)
	os.Stdout.Write(b)
	_ = os.Mkdir("flow", 0700)
	err = ioutil.WriteFile(fmt.Sprintf("flow/%v.go",
		strings.ToLower(flow.Flow.Name)),
		b,
		os.ModePerm)
	if err != nil {
		fmt.Printf("Failed to flush file to disc %v\n", err)
		os.Exit(1)
	}
}

func templateCoder(templateName string,
	templateString string,
	data interface{}) ([]byte, error) {
	//fmt.Printf("Context is %v\n", data)
	funcMap := template.FuncMap{
		"tolower": func(s string) string {
			return strings.ToLower(s)
		},
		"toStructPack": func(c string) string {
			return c + "_pack"
		},
	}
	t := template.New(templateName).Funcs(funcMap)
	t, err := t.Parse(templateString)
	if err != nil {
		fmt.Printf("Issue parsing template %v\n", err)
		os.Exit(1)
	}
	var buffer bytes.Buffer
	err = t.Execute(&buffer, data)
	if err != nil {
		fmt.Printf("Issue while applying template %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Generated code %v\n", buffer.String())
	//Format the output that's generated
	return format.Source(buffer.Bytes())
}

const contextCode string = `
//Context ...
type {{ .Name }}Context struct {
	 	{{ range $key, $value := .Data }}
	 	{{ $key }} {{ $value }}
	 	{{ end }}
	 }
`

const clientCode string = `
//{{ .Flow.Name }}FlowClient ...
type {{ .Flow.Name }}FlowClient interface {
	StartFlow(accountID string, context {{ .Flow.Name }}Context) (models.RunID, error)
    GetFlow(id models.RunID) (models.WorkflowRun, error)
}

{{$flowclientstruct := (toStructPack (tolower .Flow.Name))}}

func New{{ .Flow.Name }}FlowBuilder(dbCommon *db.Common) {{ .Flow.Name }}FlowBuilder {
	return {{ .Flow.Name }}FlowBuilder{
        flow: {{ $flowclientstruct }}flow {
		        handlerMap: make(map [string]steps.StepHandler),
		        flowRunner: workflow.NewFlowRunner(dbCommon),
	}}
}

//{{ .Flow.Name }}FlowBuilder ...
type {{ .Flow.Name }}FlowBuilder struct {
	flow {{ $flowclientstruct }}flow
}

{{ $flowname := .Flow.Name}}
{{ range .Steps }}
//With{{.Name}}Handler ...
func (cfb *{{ $flowname }}FlowBuilder) With{{ .Name }}Handler(handler steps.StepHandler) *{{ $flowname }}FlowBuilder {
	cfb.flow.handlerMap[handler.Name()] = handler
	return cfb
}
{{ end }}

//Build ...
func (cfb *{{ .Flow.Name }}FlowBuilder) Build() {{ .Flow.Name }}FlowClient {
	return &cfb.flow
}

type {{ $flowclientstruct }}flow struct {
	handlerMap steps.HandlerMap
	flowRunner workflow.FlowRunner
}

func (cu *{{ $flowclientstruct }}flow) StartFlow(accountID string, context {{ .Flow.Name }}Context) (models.RunID, error) {
	runner := cu.flowRunner
	hMapRef := &cu.handlerMap
	flow := {{ (tolower .Flow.Name) }}{}
    var contextMap models.FlowContext
	err := mapstructure.Decode(context, &contextMap)
	if err != nil {
		return "", err
	}
	return runner.StartRun(accountID, flow, contextMap, hMapRef)
}

func (cu *{{ $flowclientstruct }}flow) GetFlow(id models.RunID) (models.WorkflowRun, error) {
	runner := cu.flowRunner
	return runner.GetWorkflowRun(id)
}

{{ $flowname := .Flow.Name}}
{{ range .Steps }}
func New{{.Name}}Handler() {{.Name}}Handler {
	return {{.Name}}Handler{
		
	}
}

type {{.Name}}Handler struct{

}

func ({{.Name}}Handler) Name() string {
	return "{{.Name}}"
}

func (c {{.Name}}Handler) Run(flowContext models.FlowContext) (models.FlowContext, error) {
	var context {{ $flowname }}Context
	err := mapstructure.Decode(flowContext, &context)
	if err != nil {
		return flowContext, err
	}
	err = c.run(&context)
	if err != nil {
		return nil, err
	}
	err = mapstructure.Decode(context, &flowContext)
	return flowContext, err
}

func (c {{.Name}}Handler) Status(flowContext models.FlowContext) (steps.StepStatus, error) {
	var context {{ $flowname }}Context
	err := mapstructure.Decode(flowContext, &context)
	if err != nil {
		return steps.StepFailed, err
	}
	return c.status(&context)
}

func (c {{.Name}}Handler) run(_ *{{ $flowname }}Context) error {
	panic("Not implemented!!!!")
}

func (c {{.Name}}Handler) status(_ *{{ $flowname }}Context) (steps.StepStatus, error) {
	panic("Not implemented!!!!!")
}
{{ end }}
{{ $flowpackagename := (tolower $flowname) }}
func New{{$flowname}}FlowClient() {{ $flowpackagename }}.{{ $flowname }}FlowClient {
	builder := {{ $flowpackagename }}.New{{ $flowname }}FlowBuilder(dbCommon)
	{{ range .Steps }}
	{{.Name}}Handler := {{ $flowpackagename }}.New{{.Name}}Handler()
	builder.With{{.Name}}Handler({{.Name}}Handler)
	{{ end }}
	return builder.Build()
}

`
