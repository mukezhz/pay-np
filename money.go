package paynp

import (
	"fmt"
	"strconv"
	"strings"
)

// Paisa is an amount in NPR minor units (1 rupee = 100 paisa). Integer money
// avoids float rounding; providers that speak rupees convert at the edge.
type Paisa int64

// Rupees formats as "123.45".
func (p Paisa) Rupees() string {
	sign := ""
	if p < 0 {
		sign, p = "-", -p
	}
	return fmt.Sprintf("%s%d.%02d", sign, p/100, p%100)
}

// ParseRupees parses provider rupee strings such as "100", "133.0", "1,000.50",
// "10.0000". Non-zero digits past the paisa are rejected.
func ParseRupees(s string) (Paisa, error) {
	s = strings.ReplaceAll(strings.TrimSpace(s), ",", "")
	whole, frac, _ := strings.Cut(s, ".")
	if len(frac) > 2 && strings.TrimRight(frac[2:], "0") == "" {
		frac = frac[:2]
	}
	if strings.TrimLeft(whole, "+-") == "" || len(frac) > 2 {
		return 0, fmt.Errorf("paynp: invalid rupee amount %q", s)
	}
	frac = (frac + "00")[:2]
	n, err := strconv.ParseInt(whole+frac, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("paynp: invalid rupee amount %q: %w", s, err)
	}
	return Paisa(n), nil
}
