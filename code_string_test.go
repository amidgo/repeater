package retry

import "testing"

func Test_code_String(t *testing.T) {
	defer func() {
		msg := recover().(string)
		expectedMsg := "unexpected code value: 100"

		if msg != expectedMsg {
			t.Fatalf("compare recover msg, msgs not equal\n\nexpected:\n%s\n\nactual:\n%s", expectedMsg, msg)
		}
	}()

	c := code(100)

	_ = c.String()
}
