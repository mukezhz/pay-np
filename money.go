package paynp

import (
	"fmt"
	"strconv"
	"strings"
)

// Paisa is NPR in minor units (100 paisa = 1 rupee).
type Paisa int64

func (p Paisa) Rupees() string {
	sign := ""
	if p < 0 {
		sign, p = "-", -p
	}
	return fmt.Sprintf("%s%d.%02d", sign, p/100, p%100)
}

// ParseRupees accepts "100", "133.0", "1,000.50" and "10.0000"; sub-paisa is rejected.
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
