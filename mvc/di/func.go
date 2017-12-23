package di

import (
	"fmt"
	"reflect"
)

type (
	targetFuncInput struct {
		Object     *BindObject
		InputIndex int
	}

	// FuncInjector keeps the data that are needed in order to do the binding injection
	// as fast as possible and with the best possible and safest way.
	FuncInjector struct {
		// the original function, is being used
		// only the .Call, which is referring to the same function, always.
		fn reflect.Value

		inputs []*targetFuncInput
		// Length is the number of the valid, final binded input arguments.
		Length int
		// Valid is True when `Length` is > 0, it's statically set-ed for
		// performance reasons.
		Valid bool

		trace string // for debug info.
	}
)

// MakeFuncInjector returns a new func injector, which will be the object
// that the caller should use to bind input arguments of the "fn" function.
//
// The hijack and the goodFunc are optional, the "values" is the dependencies values.
func MakeFuncInjector(fn reflect.Value, hijack Hijacker, goodFunc TypeChecker, values ...reflect.Value) *FuncInjector {
	typ := IndirectType(fn.Type())
	s := &FuncInjector{
		fn: fn,
	}

	if !IsFunc(typ) {
		return s
	}

	n := typ.NumIn()

	// function input can have many values of the same types,
	// so keep track of them in order to not set a func input to a next bind value,
	// i.e (string, string) with two different binder funcs because of the different param's name.
	consumedValues := make(map[int]bool, n)

	for i := 0; i < n; i++ {
		inTyp := typ.In(i)

		if hijack != nil {
			if b, ok := hijack(inTyp); ok && b != nil {
				s.inputs = append(s.inputs, &targetFuncInput{
					InputIndex: i,
					Object:     b,
				})
				continue
			}
		}

		for valIdx, val := range values {
			if _, shouldSkip := consumedValues[valIdx]; shouldSkip {
				continue
			}
			inTyp := typ.In(i)

			// the binded values to the func's inputs.
			b, err := MakeBindObject(val, goodFunc)

			if err != nil {
				return s // if error stop here.
			}

			if b.IsAssignable(inTyp) {
				// println(inTyp.String() + " is assignable to " + val.Type().String())
				// fmt.Printf("binded input index: %d for type: %s and value: %v with pointer: %v\n",
				// 	i, b.Type.String(), val.String(), val.Pointer())
				s.inputs = append(s.inputs, &targetFuncInput{
					InputIndex: i,
					Object:     &b,
				})

				consumedValues[valIdx] = true
				break
			}
		}
	}

	s.Length = len(s.inputs)
	s.Valid = s.Length > 0

	for i, in := range s.inputs {
		bindmethodTyp := bindTypeString(in.Object.BindType)
		typIn := typ.In(in.InputIndex)
		// remember: on methods that are part of a struct (i.e controller)
		// the input index  = 1 is the begggining instead of the 0,
		// because the 0 is the controller receiver pointer of the method.
		s.trace += fmt.Sprintf("[%d] %s binding: '%s' for input position: %d and type: '%s'\n", i+1, bindmethodTyp, in.Object.Type.String(), in.InputIndex, typIn.String())
	}

	return s
}

// String returns a debug trace text.
func (s *FuncInjector) String() string {
	return s.trace
}

// Inject accepts an already created slice of input arguments
// and fills them, the "ctx" is optional and it's used
// on the dependencies that depends on one or more input arguments, these are the "ctx".
func (s *FuncInjector) Inject(in *[]reflect.Value, ctx ...reflect.Value) {
	args := *in
	for _, input := range s.inputs {
		input.Object.Assign(ctx, func(v reflect.Value) {
			// fmt.Printf("assign input index: %d for value: %v\n",
			// 	input.InputIndex, v.String())
			args[input.InputIndex] = v
		})

	}

	*in = args
}

// Call calls the "Inject" with a new slice of input arguments
// that are computed by the length of the input argument from the MakeFuncInjector's "fn" function.
//
// If the function needs a receiver, so
// the caller should be able to in[0] = receiver before injection,
// then the `Inject` method should be used instead.
func (s *FuncInjector) Call(ctx ...reflect.Value) []reflect.Value {
	in := make([]reflect.Value, s.Length, s.Length)
	s.Inject(&in, ctx...)
	return s.fn.Call(in)
}
