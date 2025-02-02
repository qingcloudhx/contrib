package string

import (
	"strings"

	"flogo/core/data"
	"flogo/core/data/expression/function"
)

func init() {
	function.Register(&fnToUpper{})
}

type fnToUpper struct {
}

func (fnToUpper) Name() string {
	return "toUpper"
}

func (fnToUpper) Sig() (paramTypes []data.Type, isVariadic bool) {
	return []data.Type{data.TypeString}, false
}

func (fnToUpper) Eval(params ...interface{}) (interface{}, error) {
	return strings.ToUpper(params[0].(string)), nil
}
