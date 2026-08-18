package workflow

import "fmt"

type ErrorType string

const (
	ErrorTransient  ErrorType = "TRANSIENT"
	ErrorPermanent  ErrorType = "PERMANENT"
	ErrorValidation ErrorType = "VALIDATION"
)

type WorkflowError struct {
	Type    ErrorType
	Code    string
	Message string
	Err     error
}

func (e *WorkflowError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf(
			"type=%s code=%s message=%s cause=%v",
			e.Type,
			e.Code,
			e.Message,
			e.Err,
		)
	}

	return fmt.Sprintf(
		"type=%s code=%s message=%s",
		e.Type,
		e.Code,
		e.Message,
	)
}

func NewTransientError(code, message string, err error) error {
	return &WorkflowError{
		Type:    ErrorTransient,
		Code:    code,
		Message: message,
		Err:     err,
	}
}

func NewPermanentError(code, message string, err error) error {
	return &WorkflowError{
		Type:    ErrorPermanent,
		Code:    code,
		Message: message,
		Err:     err,
	}
}

func NewValidationError(code, message string, err error) error {
	return &WorkflowError{
		Type:    ErrorValidation,
		Code:    code,
		Message: message,
		Err:     err,
	}
}
