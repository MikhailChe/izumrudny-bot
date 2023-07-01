package tracer

import (
	"encoding/json"
	"fmt"
	"runtime"
	"time"
)

type traceCtx struct {
	rootSpan    *span
	currentSpan *span
}

var globaltracectx traceCtx = traceCtx{}

func (t *traceCtx) Start(name string) *span {
	newSpan := &span{
		Name: name, Start: time.Now(), parent: t.currentSpan,
	}
	if t.currentSpan == nil {
		t.rootSpan = nil
	}
	if t.currentSpan != nil {
		t.currentSpan.Child = append(t.currentSpan.Child, newSpan)
	}
	t.currentSpan = newSpan
	if t.rootSpan == nil {
		t.rootSpan = newSpan
	}
	return newSpan
}
func (t *traceCtx) Finish(s *span) {
	s.Finish = time.Now()
	s.Elapsed = time.Since(s.Start).String()
	t.currentSpan = s.parent
}

type span struct {
	Name    string
	Start   time.Time
	Finish  time.Time
	Elapsed string
	Child   []*span
	parent  *span `json:"-"`
}

func Trace(opName string) func() {
	pc, file, line, ok := runtime.Caller(1)
	if ok && opName == "" {
		opName = fmt.Sprintf("%s:%d//%s", file, line, runtime.FuncForPC(pc).Name())
	}
	span := globaltracectx.Start(opName)
	return func() {
		globaltracectx.Finish(span)
	}
}

func PrintTrace() ([]byte, error) {
	if globaltracectx.rootSpan != nil {
		if globaltracectx.rootSpan.Elapsed == "" {
			globaltracectx.rootSpan.Elapsed = time.Since(globaltracectx.rootSpan.Start).String() + " not finished"
		}
	}
	return json.MarshalIndent(ChromeTraceEvents(globaltracectx.rootSpan), " ", " ")
}

type chromeTrace struct {
	TraceEvents chromeTraceEvents `json:"traceEvents"`
}

type chromeTraceEvents []chromeTraceEvent

type chromeTraceEvent struct {
	PID  int    `json:"pid"`
	TID  int    `json:"tid"`
	Ts   int64  `json:"ts"`  // microsedonds
	Dur  int64  `json:"dur"` //microsecnds
	PH   string `json:"ph"`  // X - завершенный
	Name string `json:"name"`
	Args any    `json:"args,omitempty"`
}

func ChromeTraceEvents(root *span) chromeTrace {
	if root == nil {
		return chromeTrace{}
	}
	var queue []*span
	startTS := root.Start
	var chromeEvents chromeTraceEvents
	queue = append(queue, root)
	for len(queue) > 0 {
		var span = queue[0]

		start := span.Start
		finish := span.Finish
		if finish.IsZero() {
			finish = time.Now()
		}

		var event = chromeTraceEvent{
			PID:  1,
			TID:  1,
			Ts:   start.Sub(startTS).Microseconds(),
			Dur:  finish.Sub(start).Microseconds(),
			PH:   "X",
			Name: span.Name,
		}
		chromeEvents = append(chromeEvents, event)

		queue = queue[1:]
		queue = append(queue, span.Child...)
	}
	return chromeTrace{chromeEvents}
}
