package domain

import "slices"

var validTransitions = map[OrderStatus][]OrderStatus{
	StatusCreated: {
		StatusAccepted,
		StatusRejected,
		StatusCancelled,
	},
	StatusAccepted: {
		StatusCooking,
		StatusCancelled,
	},
	StatusCooking: {
		StatusReady,
	},
	StatusReady: {
		StatusCompleted,
	},
	StatusRejected:  {},
	StatusCompleted: {},
	StatusCancelled: {},
}

func CanTransition(from, to OrderStatus) bool {
	allowed, ok := validTransitions[from]
	if !ok {
		return false
	}

	return slices.Contains(allowed, to)
}
