// Package common provides common utility functions for error handling, formatting, and multi-error management.
package common

import (
	"errors"
	"fmt"

	"github.com/drunkleen/l-ui/internal/logger"
)

type CodedError interface {
	error
	CodeValue() string
}

type codedError struct {
	code string
	err  error
}

func (e *codedError) Error() string     { return e.err.Error() }
func (e *codedError) Unwrap() error     { return e.err }
func (e *codedError) CodeValue() string { return e.code }

func NewCodedError(code string, err error) error {
	if err == nil {
		return nil
	}
	return &codedError{code: code, err: err}
}

func ErrorCode(err error) string {
	var ce CodedError
	if errors.As(err, &ce) {
		return ce.CodeValue()
	}
	return ""
}

// NewErrorf creates a new error with formatted message.
func NewErrorf(format string, a ...any) error {
	msg := fmt.Sprintf(format, a...)
	return errors.New(msg)
}

// NewError creates a new error from the given arguments.
func NewError(a ...any) error {
	msg := fmt.Sprintln(a...)
	return errors.New(msg)
}

// Recover handles panic recovery and logs the panic error if a message is provided.
func Recover(msg string) any {
	panicErr := recover()
	if panicErr != nil {
		if msg != "" {
			logger.Error(msg, "panic:", panicErr)
		}
	}
	return panicErr
}
