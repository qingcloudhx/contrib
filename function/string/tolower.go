package string

import (
	"strings"

	"flogo/core/data"
	"flogo/core/data/expression/function"
)

func init() {
	function.Register(&fnToLower{})
}

type fnToLower struct {
}

func (fnToLower) Name() string {
	return "toLower"
}

func (fnToLower) Sig() (paramTypes []data.Type, isVariadic bool) {
	return []data.Type{data.TypeString}, false
}

func (fnToLower) Eval(params ...interface{}) (interface{}, error) {
	return strings.ToLower(params[0].(string)), nil
}
