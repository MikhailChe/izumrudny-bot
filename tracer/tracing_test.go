package tracer

import (
	"fmt"
	"testing"
	"time"
)

func TestTracing(t *testing.T) {
	defer Trace("TestTracing")()
	A1()
	trace, err := PrintTrace()
	if err != nil {
		t.Logf("error printing trace: %v", err)
		t.Fail()
	}
	t.Log(string(trace))

	t.Log(fmt.Sprintf("%#v", ChromeTraceEvents(globaltracectx.rootSpan)))
}

func A1() {
	defer Trace("A1")()
	A2()
}

func A2() {
	defer Trace("A2")()
	time.Sleep(1 * time.Second)
}
