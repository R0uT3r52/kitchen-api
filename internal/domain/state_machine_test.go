package domain

import "testing"

func TestCanTransition(t *testing.T) {
	tests := []struct {
		name     string
		from     OrderStatus
		to       OrderStatus
		expected bool
	}{
		// From Created
		{"created to accepted", StatusCreated, StatusAccepted, true},
		{"created to rejected", StatusCreated, StatusRejected, true},
		{"created to cancelled", StatusCreated, StatusCancelled, true},
		{"created to cooking", StatusCreated, StatusCooking, false},
		{"created to ready", StatusCreated, StatusReady, false},
		{"created to completed", StatusCreated, StatusCompleted, false},

		// From Accepted
		{"accepted to cooking", StatusAccepted, StatusCooking, true},
		{"accepted to cancelled", StatusAccepted, StatusCancelled, true},
		{"accepted to ready", StatusAccepted, StatusReady, false},
		{"accepted to rejected", StatusAccepted, StatusRejected, false},
		{"accepted to created", StatusAccepted, StatusCreated, false},

		// From Cooking
		{"cooking to ready", StatusCooking, StatusReady, true},
		{"cooking to cancelled", StatusCooking, StatusCancelled, false},
		{"cooking to completed", StatusCooking, StatusCompleted, false},

		// From Ready
		{"ready to completed", StatusReady, StatusCompleted, true},
		{"ready to cancelled", StatusReady, StatusCancelled, false},

		// Final statuses cannot transition
		{"rejected to accepted", StatusRejected, StatusAccepted, false},
		{"completed to created", StatusCompleted, StatusCreated, false},
		{"cancelled to accepted", StatusCancelled, StatusAccepted, false},
		{"cancelled to cooking", StatusCancelled, StatusCooking, false},

		// Unknown status
		{"unknown to created", OrderStatus("unknown"), StatusCreated, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CanTransition(tt.from, tt.to)
			if got != tt.expected {
				t.Errorf("CanTransition(%s, %s) = %v; want %v", tt.from, tt.to, got, tt.expected)
			}
		})
	}
}
