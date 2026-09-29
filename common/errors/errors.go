// Package errors is a drop-in replacement for Golang lib 'errors'.
package errors

import (
	"context"
	stderrors "errors"
	"runtime"
	"strings"

	c "github.com/xtls/xray-core/common/ctx"
	"github.com/xtls/xray-core/common/log"
	"github.com/xtls/xray-core/common/serial"
)

const trim = len("github.com/xtls/xray-core/")

type hasInnerError interface {
	// Unwrap returns the underlying error of this one.
	Unwrap() error
}

// Error is an error object with underlying error.
type Error struct {
	prefix  []any
	message []any
	caller  string
	inner   error
}

// Error implements error.Error().
func (err *Error) Error() string {
	builder := strings.Builder{}
	for _, prefix := range err.prefix {
		builder.WriteByte('[')
		builder.WriteString(serial.ToString(prefix))
		builder.WriteString("] ")
	}

	if len(err.caller) > 0 {
		builder.WriteString(err.caller)
		builder.WriteString(": ")
	}

	msg := serial.Concat(err.message...)
	builder.WriteString(msg)

	if err.inner != nil {
		builder.WriteString(" > ")
		builder.WriteString(err.inner.Error())
	}

	return builder.String()
}

// Unwrap implements hasInnerError.Unwrap()
func (err *Error) Unwrap() error {
	if err.inner == nil {
		return nil
	}
	return err.inner
}

func (err *Error) Base(e error) *Error {
	err.inner = e
	return err
}

// String returns the string representation of this error.
func (err *Error) String() string {
	return err.Error()
}

type ExportOptionHolder struct {
	SessionID uint32
}

type ExportOption func(*ExportOptionHolder)

// New returns a new error object with message formed from given arguments.
func New(msg ...any) *Error {
	pc, _, _, _ := runtime.Caller(1) //nolint:dogsled // runtime.Caller returns 4 values, only the PC is used
	details := runtime.FuncForPC(pc).Name()
	if len(details) >= trim {
		details = details[trim:]
	}
	i := strings.Index(details, ".")
	if i > 0 {
		details = details[:i]
	}
	return &Error{
		message: msg,
		caller:  details,
	}
}

func LogDebug(ctx context.Context, msg ...any) {
	doLog(ctx, nil, log.Severity_Debug, msg...)
}

func LogDebugInner(ctx context.Context, inner error, msg ...any) {
	doLog(ctx, inner, log.Severity_Debug, msg...)
}

func LogInfo(ctx context.Context, msg ...any) {
	doLog(ctx, nil, log.Severity_Info, msg...)
}

func LogInfoInner(ctx context.Context, inner error, msg ...any) {
	doLog(ctx, inner, log.Severity_Info, msg...)
}

func LogWarning(ctx context.Context, msg ...any) {
	doLog(ctx, nil, log.Severity_Warning, msg...)
}

func LogWarningInner(ctx context.Context, inner error, msg ...any) {
	doLog(ctx, inner, log.Severity_Warning, msg...)
}

func LogError(ctx context.Context, msg ...any) {
	doLog(ctx, nil, log.Severity_Error, msg...)
}

func LogErrorInner(ctx context.Context, inner error, msg ...any) {
	doLog(ctx, inner, log.Severity_Error, msg...)
}

func doLog(ctx context.Context, inner error, severity log.Severity, msg ...any) {
	if log.GetSeverity() < severity {
		return
	}
	pc, _, _, _ := runtime.Caller(2) //nolint:dogsled // runtime.Caller returns 4 values, only the PC is used
	details := runtime.FuncForPC(pc).Name()
	if len(details) >= trim {
		details = details[trim:]
	}
	i := strings.Index(details, ".")
	if i > 0 {
		details = details[:i]
	}
	err := &Error{
		message: msg,
		caller:  details,
		inner:   inner,
	}
	if ctx != nil && ctx != context.Background() {
		id := uint32(c.IDFromContext(ctx))
		if id > 0 {
			err.prefix = append(err.prefix, id)
		}
	}
	log.Record(&log.GeneralMessage{
		Severity: severity,
		Content:  err,
	})
}

// Is reports whether any error in err's chain matches target.
//
// It is a thin wrapper over the standard library, so that the many callers that
// already import this package as `errors` do not need a second import for the
// idiomatic `errors.Is(err, target)` form.
func Is(err, target error) bool {
	return stderrors.Is(err, target)
}

// As finds the first error in err's chain that matches target, and if one is
// found, sets target to that error value and returns true.
//
// See Is for why this package re-exports the standard library function.
func As(err error, target any) bool {
	return stderrors.As(err, target)
}

// AsType finds the first error in err's chain that matches the type parameter
// and returns it. The second result reports whether a match was found.
//
// See Is for why this package re-exports the standard library function.
func AsType[E error](err error) (E, bool) {
	return stderrors.AsType[E](err)
}

// Cause returns the root cause of this error.
func Cause(err error) error {
	if err == nil {
		return nil
	}
L:
	for {
		switch inner := err.(type) { //nolint:errorlint // this switch *is* the chain walk that errors.As would perform
		case hasInnerError:
			if inner.Unwrap() == nil {
				break L
			}
			err = inner.Unwrap()
		default:
			break L
		}
	}
	return err
}
