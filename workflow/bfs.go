package workflow

func bfs(buffer []interface{},
	processFn func(interface{}),
	getNextFn func(interface{}) []interface{}) {
	for len(buffer) > 0 {
		element := buffer[0]
		processFn(element)
		nextElements := getNextFn(element)
		buffer = buffer[1:]
		if len(nextElements) > 0 {
			buffer = append(buffer, nextElements ...)
		}
	}
}
