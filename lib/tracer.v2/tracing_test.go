package tracer

import (
	"context"
	"testing"
	"time"
)

func TestTracing(t *testing.T) {
	ctx := context.Background()
	ctx, span := Open(ctx, Named("TestTracing"))
	defer span.Close()
	A1(ctx)
	trace, err := span.PrintTrace()
	if err != nil {
		t.Logf("error printing trace: %v", err)
		t.Fail()
	}
	t.Log(string(trace))

	t.Logf("%#v", span.chromeTraceEvents())
}

func A1(ctx context.Context) {
	ctx, span := Open(ctx, Named("A1"))
	defer span.Close()
	A2(ctx)
}

func A2(ctx context.Context) {
	ctx, span := Open(ctx, Named("A2"))
	defer span.Close()
	time.Sleep(1 * time.Second)
}
