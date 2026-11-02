package domain

import "errors"

type ErrorKind string

const (
	KindValidation       ErrorKind = "VALIDATION_ERROR"
	KindNotFound         ErrorKind = "NOT_FOUND"
	KindConflict         ErrorKind = "CONFLICT"
	KindAuthentication   ErrorKind = "AUTHENTICATION_FAILED"
	KindForbidden        ErrorKind = "FORBIDDEN"
	KindUnsupportedMedia ErrorKind = "UNSUPPORTED_MEDIA_TYPE"
	KindMalformedBody    ErrorKind = "MALFORMED_BODY"
	KindPayloadTooLarge  ErrorKind = "PAYLOAD_TOO_LARGE"
)

const (
	messageNotFound           = "not found"
	messageForbidden          = "forbidden"
	messageTaken              = "has already been taken"
	messageMissingToken       = "is missing"
	messageInvalidToken       = "is invalid"
	messageExpiredToken       = "has expired"
	messageInvalidCredentials = "are invalid"
)

type Error struct {
	Kind   ErrorKind
	Fields map[string][]string
}

func (e *Error) Error() string {
	return string(e.Kind)
}

func newError(kind ErrorKind, key string, messages ...string) *Error {
	return &Error{Kind: kind, Fields: map[string][]string{key: messages}}
}

func NewValidationError(fields map[string][]string) error {
	return &Error{Kind: KindValidation, Fields: fields}
}

func NewFieldError(field string, messages ...string) error {
	return newError(KindValidation, field, messages...)
}

func NewConflictError(fields ...string) error {
	conflict := &Error{Kind: KindConflict, Fields: map[string][]string{}}
	for _, field := range fields {
		conflict.Fields[field] = []string{messageTaken}
	}
	return conflict
}

func NewNotFoundError(resource string) error {
	return newError(KindNotFound, resource, messageNotFound)
}

func NewForbiddenError(resource string) error {
	return newError(KindForbidden, resource, messageForbidden)
}

func NewMissingTokenError() error {
	return newError(KindAuthentication, "token", messageMissingToken)
}

func NewInvalidTokenError() error {
	return newError(KindAuthentication, "token", messageInvalidToken)
}

func NewExpiredTokenError() error {
	return newError(KindAuthentication, "token", messageExpiredToken)
}

func NewInvalidCredentialsError() error {
	return newError(KindAuthentication, "credentials", messageInvalidCredentials)
}

func NewUnsupportedMediaTypeError() error {
	return newError(KindUnsupportedMedia, "content-type", "must be application/json")
}

func NewMalformedBodyError() error {
	return newError(KindMalformedBody, "body", "is not valid JSON")
}

func NewPayloadTooLargeError() error {
	return newError(KindPayloadTooLarge, "body", "is too large")
}

func KindOf(err error) (ErrorKind, bool) {
	var domainErr *Error
	if errors.As(err, &domainErr) {
		return domainErr.Kind, true
	}
	return "", false
}

func IsKind(err error, kind ErrorKind) bool {
	actual, ok := KindOf(err)
	return ok && actual == kind
}

func FieldsOf(err error) map[string][]string {
	var domainErr *Error
	if errors.As(err, &domainErr) {
		return domainErr.Fields
	}
	return nil
}

type ErrorCollector struct {
	fields map[string][]string
}

func NewErrorCollector() *ErrorCollector {
	return &ErrorCollector{fields: map[string][]string{}}
}

func (c *ErrorCollector) Add(field, message string) {
	c.fields[field] = append(c.fields[field], message)
}

func (c *ErrorCollector) Capture(err error) error {
	if err == nil {
		return nil
	}
	if !IsKind(err, KindValidation) {
		return err
	}
	for field, messages := range FieldsOf(err) {
		for _, message := range messages {
			c.Add(field, message)
		}
	}
	return nil
}

func (c *ErrorCollector) IsEmpty() bool {
	return len(c.fields) == 0
}

func (c *ErrorCollector) Err() error {
	if c.IsEmpty() {
		return nil
	}
	return NewValidationError(c.fields)
}
