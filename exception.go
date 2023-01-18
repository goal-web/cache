package cache

import "github.com/goal-web/contracts"

type DriverException struct {
	error
	fields contracts.Fields
}

func (exception DriverException) Error() string {
	return exception.error.Error()
}

func (exception DriverException) Fields() contracts.Fields {
	return exception.fields
}
