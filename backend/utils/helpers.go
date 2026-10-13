package utils

import "strconv"

// Itoa переводит int в строку (обёртка над strconv.Itoa для удобства)
func Itoa(n int) string {
	return strconv.Itoa(n)
}

// JoinStrings соединяет строки через разделитель
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

// Round2 округляет float64 до двух знаков после запятой
func Round2(v float64) float64 {
	return float64(int(v*100+0.5)) / 100
}
