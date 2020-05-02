package controller

import (
	"github.com/pgbouncer-operator/pgbouncer-operator/pkg/controller/pgbouncer"
)

func init() {
	// AddToManagerFuncs is a list of functions to create controllers and add them to a manager.
	AddToManagerFuncs = append(AddToManagerFuncs, pgbouncer.Add)
}
