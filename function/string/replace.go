package string

import (
	"strings"

	"flogo/core/data"
	"flogo/core/data/expression/function"
)

func init() {
	function.Register(&fnReplace{})
}

type fnReplace struct {
}

func (fnReplace) Name() string {
	return "replace"
}

func (fnReplace) Sig() (paramTypes []data.Type, isVariadic bool) {
	return []data.Type{data.TypeString, data.TypeString, data.TypeString, data.TypeInt}, false
}

func (fnReplace) Eval(params ...interface{}) (interface{}, error) {
	return strings.Replace(params[0].(string), params[1].(string), params[2].(string), params[3].(int)), nil
}
