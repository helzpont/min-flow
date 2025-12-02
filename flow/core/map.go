package core

import "fmt"

// Mapper defines a function that maps a Result of type IN to a Result of type OUT. It represents a transformation
// that maintains the cardinality of the flow (one input item produces one output item).
// The mapper function is at the lowest level of abstraction in the flow processing pipeline.
// It answers the question: "What is done to each item in the flow?"
type Mapper[IN, OUT any] func(*Result[IN]) (*Result[OUT], error)

func Map[IN, OUT any](mapFunc func(IN) (OUT, error)) Mapper[IN, OUT] {
	return func(res *Result[IN]) (out *Result[OUT], err error) {
		defer func() {
			if r := recover(); r != nil {
				err = fmt.Errorf("panic in Map function: %v", r)
			}
		}()

		if res.IsError() {
			return Err[OUT](res.Error()), nil
		}
		mappedValue, err := mapFunc(res.Value())
		if err != nil {
			return Err[OUT](err), nil
		}
		return Ok(mappedValue), nil
	}
}

// FlatMapper defines a function that maps a Result of type IN to a Result containing a slice of Results of type OUT.
// It represents a transformation that can change the cardinality of the flow (one input item can produce zero or more output items).
// The flat mapper function is at the lowest level of abstraction in the flow processing pipeline.
// It answers the question: "How are items in the flow reduced or expanded?"
type FlatMapper[IN, OUT any] func(*Result[IN]) ([]*Result[OUT], error)

func FlatMap[IN, OUT any](flatMapFunc func(IN) ([]OUT, error)) FlatMapper[IN, OUT] {
	return func(res *Result[IN]) (outs []*Result[OUT], err error) {
		defer func() {
			if r := recover(); r != nil {
				err = fmt.Errorf("panic in FlatMap function: %v", r)
			}
		}()

		if res.IsError() {
			return []*Result[OUT]{Err[OUT](res.Error())}, nil
		}
		mappedValues, err := flatMapFunc(res.Value())
		if err != nil {
			return []*Result[OUT]{Err[OUT](err)}, nil
		}
		results := make([]*Result[OUT], len(mappedValues))
		for i, v := range mappedValues {
			results[i] = Ok(v)
		}
		return results, nil
	}
}
