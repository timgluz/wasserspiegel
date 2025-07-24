package station

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMeasurementDifference(t *testing.T) {
	testCases := []struct {
		name        string
		current     Measurement
		previous    Measurement
		expected    Measurement
		expectError bool
	}{
		{
			name:        "increase 5 cm",
			current:     Measurement{Timestamp: "2023-10-01T12:00:00Z", Value: 10.0, Unit: "cm"},
			previous:    Measurement{Timestamp: "2023-10-01T11:00:00Z", Value: 5.0, Unit: "cm"},
			expected:    Measurement{Timestamp: "2023-10-01T12:00:00Z", Value: 5.0, Unit: "cm"},
			expectError: false,
		},
		{
			name:        "decrease 3 cm",
			current:     Measurement{Timestamp: "2023-10-01T12:00:00Z", Value: 7.0, Unit: "cm"},
			previous:    Measurement{Timestamp: "2023-10-01T11:00:00Z", Value: 10.0, Unit: "cm"},
			expected:    Measurement{Timestamp: "2023-10-01T12:00:00Z", Value: -3.0, Unit: "cm"},
			expectError: false,
		},
		{
			name:        "no change",
			current:     Measurement{Timestamp: "2023-10-01T12:00:00Z", Value: 10.0, Unit: "cm"},
			previous:    Measurement{Timestamp: "2023-10-01T11:00:00Z", Value: 10.0, Unit: "cm"},
			expected:    Measurement{Timestamp: "2023-10-01T12:00:00Z", Value: 0.0, Unit: "cm"},
			expectError: false,
		},
		{
			name:        "invalid previous measurement",
			current:     Measurement{Timestamp: "2023-10-01T12:00:00Z", Value: 10.0, Unit: "cm"},
			previous:    Measurement{Timestamp: "2023-10-01T11:00:00Z", Value: 0.0, Unit: "cm"}, // Invalid previous value
			expected:    Measurement{},
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := tc.current.Difference(tc.previous)
			if tc.expectError {
				assert.NotNil(t, err, "expected error for case: %s", tc.name)
				return
			}

			// replace the following check with testify assert
			assert.NoError(t, err, "unexpected error for case: %s", tc.name)
			assert.Equal(t, tc.expected.Value, result.Value, "expected value mismatch for case: %s", tc.name)
			assert.Equal(t, tc.expected.Unit, result.Unit, "expected unit mismatch for case: %s", tc.name)
		})
	}
}
