package analyzer

import (
	"fmt"

	"github.com/dop251/goja"
)

// jsByteArrayOutputStream is the narrow Rhino java.io bridge used by active cover decoders.
type jsByteArrayOutputStream struct {
	bytes []byte
}

func newJSByteArrayOutputStreamConstructor(rt *goja.Runtime) func(goja.ConstructorCall) *goja.Object {
	return func(goja.ConstructorCall) *goja.Object {
		stream := &jsByteArrayOutputStream{}
		object := rt.NewObject()
		_ = object.Set("write", stream.Write)
		_ = object.Set("toByteArray", stream.ToByteArray)
		_ = object.Set("close", stream.Close)
		return object
	}
}

func (s *jsByteArrayOutputStream) Write(value interface{}) error {
	bytes, err := ToBytes(value)
	if err == nil {
		s.bytes = append(s.bytes, bytes...)
		return nil
	}
	switch typed := value.(type) {
	case int64:
		s.bytes = append(s.bytes, byte(typed))
	case float64:
		s.bytes = append(s.bytes, byte(int64(typed)))
	default:
		return fmt.Errorf("java.io.ByteArrayOutputStream.write: %w", err)
	}
	return nil
}

func (s *jsByteArrayOutputStream) ToByteArray() []byte {
	return append([]byte(nil), s.bytes...)
}

func (s *jsByteArrayOutputStream) Close() {}
