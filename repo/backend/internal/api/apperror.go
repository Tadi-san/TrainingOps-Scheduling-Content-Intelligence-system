package api

import "errors"

type AppError struct {
	PublicMessage string
	InternalError error
}

func (e AppError) Error() string {
	if e.PublicMessage != "" {
		return e.PublicMessage
	}
	if e.InternalError != nil {
		return e.InternalError.Error()
	}
	return "request failed"
}

func WrapAppError(public string, internal error) AppError {
	return AppError{PublicMessage: public, InternalError: internal}
}

func PublicError(err error) string {
	var appErr AppError
	if errors.As(err, &appErr) {
		return appErr.PublicMessage
	}
	return "request failed"
}
