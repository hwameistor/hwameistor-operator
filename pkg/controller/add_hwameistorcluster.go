package controller

import (
	"github.com/hwameistor/hwameistor-operator/pkg/controller/hwameistorcluster"
)

func init() {
	// AddToManagerFuncs is a list of functions to create controllers and add them to a manager.
	AddToManagerFuncs = append(AddToManagerFuncs, hwameistorcluster.Add)
}
