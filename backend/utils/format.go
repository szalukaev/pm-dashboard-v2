package utils

import (
	"fmt"
	"strings"
	"time"
)

func FormatMoney(amount float64) string {
	s := fmt.Sprintf("%.2f", amount)
	parts := strings.Split(s, ".")
	intPart := parts[0]
	decPart := parts[1]

	// Add space separators for thousands
	n := len(intPart)
	if n > 3 {
		var b strings.Builder
		for i, c := range intPart {
			if i > 0 && (n-i)%3 == 0 {
				b.WriteRune(' ')
			}
			b.WriteRune(c)
		}
		intPart = b.String()
	}
	return intPart + "," + decPart + " ₽"
}

func FormatDate(t time.Time) string {
	return t.Format("02.01.2006")
}

func FormatNumber(n float64) string {
	s := fmt.Sprintf("%.2f", n)
	parts := strings.Split(s, ".")
	intPart := parts[0]
	decPart := parts[1]

	numLen := len(intPart)
	if numLen > 3 {
		var b strings.Builder
		for i, c := range intPart {
			if i > 0 && (numLen-i)%3 == 0 {
				b.WriteRune(' ')
			}
			b.WriteRune(c)
		}
		intPart = b.String()
	}
	return intPart + "," + decPart
}
