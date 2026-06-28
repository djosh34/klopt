// Package example contains generated decode examples.
package example

import "decode_and_validate_generator/pkg/peekjson"

type Decoder interface {
	Decode(decoder peekjson.Decoder) error
}
