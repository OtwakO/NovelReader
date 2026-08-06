package analyzer

import "testing"

func TestJavaByteArrayOutputStreamBridgeWritesBytes(t *testing.T) {
	value, err := NewJSVM().EvalContext(t.Context(), `
var stream = new Packages.java.io.ByteArrayOutputStream();
stream.write([65, 66]);
stream.write(67);
stream.toByteArray();
`, nil, "https://fixture.test")
	if err != nil {
		t.Fatal(err)
	}
	bytes, err := ToBytes(value)
	if err != nil || string(bytes) != "ABC" {
		t.Fatalf("bytes=%v err=%v", bytes, err)
	}
}
