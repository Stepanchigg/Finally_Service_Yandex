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
		return 0, fmt.Errorf("Невалидный оператор: %s", operation)
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
			name:      "Сложение дробных чисел",
			operation: "+",
			a:         2.5,
			b:         3.5,
			expected:  6.0,
			expectErr: false,
		},
		{
			name:      "Сложение отрицательных дробных чисел",
			operation: "+",
			a:         -2.5,
			b:         -3.5,
			expected:  -6.0,
			expectErr: false,
		},

		{
			name:      "Вычитание",
			operation: "-",
			a:         5.0,
			b:         2.5,
			expected:  2.5,
			expectErr: false,
		},
		{
			name:      "Вычитание отрицательных чисел",
			operation: "-",
			a:         -5.0,
			b:         -2.5,
			expected:  -2.5,
			expectErr: false,
		},

		{
			name:      "Умножение",
			operation: "*",
			a:         2.0,
			b:         3.0,
			expected:  6.0,
			expectErr: false,
		},
		{
			name:      "Умножение на ноль",
			operation: "*",
			a:         2.0,
			b:         0.0,
			expected:  0.0,
			expectErr: false,
		},

		{
			name:      "Деление",
			operation: "/",
			a:         6.0,
			b:         2.0,
			expected:  3.0,
			expectErr: false,
		},
		{
			name:      "Деление на ноль",
			operation: "/",
			a:         6.0,
			b:         0.0,
			expected:  0.0,
			expectErr: true,
			err:       ErrDivisionByZero,
		},

		{
			name:      "Невалидный оператор",
			operation: "<",
			a:         2.0,
			b:         3.0,
			expected:  0.0,
			expectErr: true,
			err:       fmt.Errorf("Невалидный оператор: %s", "<"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := Calculations(tt.operation, tt.a, tt.b)

			if tt.expectErr {
				if err == nil {
					t.Errorf("Ожидатаеся ошибка, получен nil")
				} else if err.Error() != tt.err.Error() {
					t.Errorf("Ожидаемая ошибка: %v, получена: %v", tt.err, err)
				}
				return
			}

			if err != nil {
				t.Errorf("Неожиданная ошибка: %v", err)
			}
			if result != tt.expected {
				t.Errorf("Ожидаемо: %v, получено: %v", tt.expected, result)
			}
		})
	}
}