package httpx

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

func IDR(v float64) string {
	negative := v < 0
	if negative {
		v = -v
	}
	intPart := int64(v + 0.5)
	s := strconv.FormatInt(intPart, 10)
	var out strings.Builder
	out.WriteString("Rp ")
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			out.WriteByte('.')
		}
		out.WriteRune(c)
	}
	if negative {
		return "-" + out.String()
	}
	return out.String()
}

func Number(v int) string {
	s := strconv.Itoa(v)
	var out strings.Builder
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			out.WriteByte('.')
		}
		out.WriteRune(c)
	}
	return out.String()
}

func DateID(t time.Time) string {
	if t.IsZero() {
		return "-"
	}
	months := []string{"", "Jan", "Feb", "Mar", "Apr", "Mei", "Jun", "Jul", "Agu", "Sep", "Okt", "Nov", "Des"}
	return fmt.Sprintf("%d %s %d", t.Day(), months[t.Month()], t.Year())
}

func DateTimeID(t time.Time) string {
	if t.IsZero() {
		return "-"
	}
	return fmt.Sprintf("%s %02d:%02d", DateID(t), t.Hour(), t.Minute())
}

// JSNumber formats a float64 as JS number literal.
func JSNumber(v float64) string {
	intPart := int64(v + 0.5)
	return strconv.FormatInt(intPart, 10)
}

// JSStringArray formats a slice of strings as JSON array literal.
func JSStringArray(arr []string) string {
	var b strings.Builder
	b.WriteByte('[')
	for i, s := range arr {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteByte('"')
		for _, r := range s {
			switch r {
			case '"', '\\':
				b.WriteByte('\\')
				b.WriteRune(r)
			default:
				b.WriteRune(r)
			}
		}
		b.WriteByte('"')
	}
	b.WriteByte(']')
	return b.String()
}

func Slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	out := make([]byte, 0, len(s))
	prevDash := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= 'a' && c <= 'z', c >= '0' && c <= '9':
			out = append(out, c)
			prevDash = false
		default:
			if !prevDash && len(out) > 0 {
				out = append(out, '-')
				prevDash = true
			}
		}
	}
	return strings.Trim(string(out), "-")
}
