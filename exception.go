package cache

import "github.com/goal-web/contracts"

type DriverException struct {
	error
	fields contracts.Fields
}

func (this DriverException) Error() string {
	return this.error.Error()
}

func (this DriverException) Fields() contracts.Fields {
	return this.fields
}
