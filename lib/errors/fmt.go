package errors

import "fmt"

func ErrorfOrNil(e error, format string, args ...any) error {
	if e == nil {
		return nil
	}
	if len(format) == 0 {
		return e
	}
	return fmt.Errorf(fmt.Sprintf(format, args...)+": %w", e)
}
