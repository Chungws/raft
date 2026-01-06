package mr

// TODO: Implement Worker

type MapFunc func(string, string) []KeyValue
type ReduceFunc func(string, []string) string

func Worker(mapf MapFunc, reducef ReduceFunc) {
	// TODO: implement
}
