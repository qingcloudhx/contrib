package string

import (
	"strings"

	"flogo/core/data"
	"flogo/core/data/expression/function"
)

func init() {
	function.Register(&fnTrimSuffix{})
}

type fnTrimSuffix struct {
}

func (fnTrimSuffix) Name() string {
	return "trimSuffix"
}

func (fnTrimSuffix) Sig() (paramTypes []data.Type, isVariadic bool) {
	return []data.Type{data.TypeString, data.TypeString}, false
}

func (fnTrimSuffix) Eval(params ...interface{}) (interface{}, error) {
	return strings.TrimSuffix(params[0].(string), params[1].(string)), nil
}
