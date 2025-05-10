package agent

import (
	"fmt"
	"testing"
)

func CalculationsForTesting(operation string, a, b float64) (float64, error) {
	switch operation {
	case "+":
		return a + b, nil
	case "-":
		return a - b, nil
	case "*":
		return a * b, nil
	case "/":
		if b == 0 {
			return 0, ErrDivisionByZero
		}
		return a / b, nil
	default:
		return 0, fmt.Errorf("invalid operator: %s", operation)
	}
}

func TestCalculations(t *testing.T) {
	tests := []struct {
		name      string
		operation string
		a, b      float64
		expected  float64
		expectErr bool
		err       error
	}{
		{
			name:      "Addition positive numbers",
			operation: "+",
			a:         2.5,
			b:         3.5,
			expected:  6.0,
			expectErr: false,
		},
		{
			name:      "Addition negative numbers",
			operation: "+",
			a:         -2.5,
			b:         -3.5,
			expected:  -6.0,
			expectErr: false,
		},

		{
			name:      "Subtraction positive numbers",
			operation: "-",
			a:         5.0,
			b:         2.5,
			expected:  2.5,
			expectErr: false,
		},
		{
			name:      "Subtraction negative numbers",
			operation: "-",
			a:         -5.0,
			b:         -2.5,
			expected:  -2.5,
			expectErr: false,
		},

		{
			name:      "Multiplication positive numbers",
			operation: "*",
			a:         2.0,
			b:         3.0,
			expected:  6.0,
			expectErr: false,
		},
		{
			name:      "Multiplication by zero",
			operation: "*",
			a:         2.0,
			b:         0.0,
			expected:  0.0,
			expectErr: false,
		},

		{
			name:      "Division positive numbers",
			operation: "/",
			a:         6.0,
			b:         2.0,
			expected:  3.0,
			expectErr: false,
		},
		{
			name:      "Division by zero",
			operation: "/",
			a:         6.0,
			b:         0.0,
			expected:  0.0,
			expectErr: true,
			err:       ErrDivisionByZero,
		},

		{
			name:      "Invalid operator",
			operation: "invalid",
			a:         2.0,
			b:         3.0,
			expected:  0.0,
			expectErr: true,
			err:       fmt.Errorf("invalid operator: %s", "invalid"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := Calculations(tt.operation, tt.a, tt.b)

			if tt.expectErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				} else if err.Error() != tt.err.Error() {
					t.Errorf("expected error: %v, got: %v", tt.err, err)
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if result != tt.expected {
				t.Errorf("expected: %v, got: %v", tt.expected, result)
			}
		})
	}
}
