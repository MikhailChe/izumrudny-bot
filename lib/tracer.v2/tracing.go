package tracer

import (
	"context"
	"encoding/json"
	"fmt"
	"runtime"
	"sync/atomic"
	"time"
)

type TSpan struct {
	children []*TSpan
	start    time.Time
	stop     time.Time
	name     string
	tid      int64
}

type SpanOptions func(*TSpan)

func WithNewTid(span *TSpan) {
	span.tid = uutid.Add(1)
}

func Named(name string) SpanOptions {
	return func(span *TSpan) {
		span.name = name
	}
}

type spanContextKeyType int

var spanContextKey spanContextKeyType

var uutid atomic.Int64

func Open(ctx context.Context, options ...SpanOptions) (context.Context, *TSpan) {
	parentSpan, _ := ctx.Value(spanContextKey).(*TSpan)
	var newSpan TSpan
	newSpan.start = time.Now()
	newSpan.name = opname()
	if parentSpan != nil {
		defer func() { parentSpan.children = append(parentSpan.children, &newSpan) }()
		newSpan.tid = parentSpan.tid
	} else {
		options = append(options, WithNewTid)
	}
	for _, opt := range options {
		opt(&newSpan)
	}
	return context.WithValue(ctx, spanContextKey, &newSpan), &newSpan
}

func WithSpan(ctx context.Context, span *TSpan) context.Context {
	return context.WithValue(ctx, spanContextKey, span)
}

func FromContext(ctx context.Context) *TSpan {
	parentSpan, _ := ctx.Value(spanContextKey).(*TSpan)
	return parentSpan
}

func Background(ctx context.Context) context.Context {
	return WithSpan(context.Background(), FromContext(ctx))
}

func (s *TSpan) Close() {
	s.stop = time.Now()
}

func (s *TSpan) PrintTrace() ([]byte, error) {
	return json.MarshalIndent(s.chromeTraceEvents(), " ", " ")
}

type chromeTrace struct {
	TraceEvents chromeTraceEvents `json:"traceEvents"`
}

type chromeTraceEvents []chromeTraceEvent

type chromeTraceEvent struct {
	PID  int    `json:"pid"`
	TID  int    `json:"tid"`
	Ts   int64  `json:"ts"`  // microsedonds
	Dur  int64  `json:"dur"` // microsecnds
	PH   string `json:"ph"`  // X - завершенный
	Name string `json:"name"`
	Args any    `json:"args,omitempty"`
}

func (s *TSpan) chromeTraceEvents() chromeTrace {
	if s == nil {
		return chromeTrace{}
	}
	var queue []*TSpan
	startTS := s.start
	var chromeEvents chromeTraceEvents
	queue = append(queue, s)
	for len(queue) > 0 {
		var span = queue[0]

		start := span.start
		finish := span.stop
		if finish.IsZero() {
			finish = time.Now()
		}

		var event = chromeTraceEvent{
			PID:  1,
			TID:  int(span.tid),
			Ts:   start.Sub(startTS).Microseconds(),
			Dur:  finish.Sub(start).Microseconds(),
			PH:   "X",
			Name: span.name,
		}
		chromeEvents = append(chromeEvents, event)

		queue = queue[1:]
		queue = append(queue, span.children...)
	}
	return chromeTrace{chromeEvents}
}

func opname() string {
	pc, _, line, ok := runtime.Caller(2)
	if ok {
		return fmt.Sprintf("%s:%d", runtime.FuncForPC(pc).Name(), line)
	}
	return ""
}
