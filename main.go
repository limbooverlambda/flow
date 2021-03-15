package main

import (

	"flag"
	"os"

	"flow/generator"
)

//TODO: Refactor to use COBRA
func main() {
	workflowJSONPath := flag.String("workflowJSON", "", "Path to the JSON workflow definition")
	flag.Parse()
	if *workflowJSONPath == "" {
		flag.Usage()
		os.Exit(1)
	}
	generator.Start(*workflowJSONPath)

}

