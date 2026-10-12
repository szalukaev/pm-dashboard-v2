package invoice

import (
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

var (
	unitsMasculine = []string{"", "один", "два", "три", "четыре", "пять", "шесть", "семь", "восемь", "девять"}
	unitsFeminine  = []string{"", "одна", "две", "три", "четыре", "пять", "шесть", "семь", "восемь", "девять"}
	teens          = []string{"десять", "одиннадцать", "двенадцать", "тринадцать", "четырнадцать", "пятнадцать",
		"шестнадцать", "семнадцать", "восемнадцать", "девятнадцать"}
	tens     = []string{"", "", "двадцать", "тридцать", "сорок", "пятьдесят", "шестьдесят", "семьдесят", "восемьдесят", "девяносто"}
	hundreds = []string{"", "сто", "двести", "триста", "четыреста", "пятьсот", "шестьсот", "семьсот", "восемьсот", "девятьсот"}
)

// plural picks the form of a noun for a number: 1 рубль, 2 рубля, 5 рублей.
func plural(n int64, one, few, many string) string {
	n %= 100
	if n >= 11 && n <= 14 {
		return many
	}
	switch n % 10 {
	case 1:
		return one
	case 2, 3, 4:
		return few
	}
	return many
}

// triad writes a number below a thousand in words.
func triad(n int, feminine bool) []string {
	var words []string
	if h := n / 100; h > 0 {
		words = append(words, hundreds[h])
	}
	rest := n % 100
	switch {
	case rest >= 10 && rest < 20:
		words = append(words, teens[rest-10])
	default:
		if t := rest / 10; t > 0 {
			words = append(words, tens[t])
		}
		if u := rest % 10; u > 0 {
			if feminine {
				words = append(words, unitsFeminine[u])
			} else {
				words = append(words, unitsMasculine[u])
			}
		}
	}
	return words
}

// integerInWords writes a whole number of rubles in words, without the noun.
func integerInWords(n int64) string {
	if n == 0 {
		return "ноль"
	}
	scales := []struct {
		one, few, many string
		feminine       bool
	}{
		{"", "", "", false},
		{"тысяча", "тысячи", "тысяч", true},
		{"миллион", "миллиона", "миллионов", false},
		{"миллиард", "миллиарда", "миллиардов", false},
	}
	var groups [][]string
	for scale := 0; n > 0 && scale < len(scales); scale++ {
		part := int(n % 1000)
		n /= 1000
		if part == 0 {
			continue
		}
		words := triad(part, scales[scale].feminine)
		if scale > 0 {
			words = append(words, plural(int64(part), scales[scale].one, scales[scale].few, scales[scale].many))
		}
		groups = append([][]string{words}, groups...)
	}
	var all []string
	for _, g := range groups {
		all = append(all, g...)
	}
	return strings.Join(all, " ")
}

// AmountInWords writes an amount of money the way invoices do:
// "Сто пять тысяч рублей 50 копеек". Kopecks stay in digits.
func AmountInWords(amount float64) string {
	if amount < 0 {
		amount = -amount
	}
	total := int64(amount*100 + 0.5)
	rubles, kopecks := total/100, total%100

	text := integerInWords(rubles) + " " + plural(rubles, "рубль", "рубля", "рублей") + " " +
		leadingZero(kopecks) + " " + plural(kopecks, "копейка", "копейки", "копеек")

	first, size := utf8.DecodeRuneInString(text)
	return string(unicode.ToUpper(first)) + text[size:]
}

func leadingZero(n int64) string {
	if n < 10 {
		return "0" + strconv.FormatInt(n, 10)
	}
	return strconv.FormatInt(n, 10)
}
