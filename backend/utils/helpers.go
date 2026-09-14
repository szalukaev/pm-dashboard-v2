package utils

import "strconv"

// Itoa converts int to string (wrapper around strconv.Itoa for convenience)
func Itoa(n int) string {
	return strconv.Itoa(n)
}

// JoinStrings concatenates strings with separator
func JoinStrings(parts []string, sep string) string {
	s := ""
	for i, p := range parts {
		if i > 0 {
			s += sep
		}
		s += p
	}
	return s
}

// Round2 rounds float64 to 2 decimal places
func Round2(v float64) float64 {
	return float64(int(v*100+0.5)) / 100
}
