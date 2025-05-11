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
		return 0, fmt.Errorf("Невалидная операция: %s", operation)
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
			name:      "Сложение позитивных чисел",
			operation: "+",
			a:         2.5,
			b:         3.5,
			expected:  6.0,
			expectErr: false,
		},
		{
			name:      "Сложение негативных чисел",
			operation: "+",
			a:         -2.5,
			b:         -3.5,
			expected:  -6.0,
			expectErr: false,
		},

		{
			name:      "Вычитание позитивных чисел",
			operation: "-",
			a:         5.0,
			b:         2.5,
			expected:  2.5,
			expectErr: false,
		},
		{
			name:      "Вычитание негативных чисел",
			operation: "-",
			a:         -5.0,
			b:         -2.5,
			expected:  -2.5,
			expectErr: false,
		},

		{
			name:      "Умножение позитивных чисел",
			operation: "*",
			a:         2.0,
			b:         3.0,
			expected:  6.0,
			expectErr: false,
		},
		{
			name:      "Умножение на нуль",
			operation: "*",
			a:         2.0,
			b:         0.0,
			expected:  0.0,
			expectErr: false,
		},

		{
			name:      "Деление позитивных чисел",
			operation: "/",
			a:         6.0,
			b:         2.0,
			expected:  3.0,
			expectErr: false,
		},
		{
			name:      "Деление на нуль",
			operation: "/",
			a:         6.0,
			b:         0.0,
			expected:  0.0,
			expectErr: true,
			err:       ErrDivisionByZero,
		},

		{
			name:      "Invalid operator",
			operation: "$",
			a:         2.0,
			b:         3.0,
			expected:  0.0,
			expectErr: true,
			err:       fmt.Errorf("invalid operator: %s", "$"),
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
