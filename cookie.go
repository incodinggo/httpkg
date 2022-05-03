package httpkg

import (
	"net/textproto"
	"strings"
)

type Cookie struct {
	Name  string
	Value string
}

func readCookies(line string) []*Cookie {
	if len(line) == 0 {
		return []*Cookie{}
	}
	cookies := make([]*Cookie, 0, len(line)+strings.Count(line, ";"))
	line = textproto.TrimString(line)

	var part string
	for len(line) > 0 { // continue since we have rest
		part, line, _ = strings.Cut(line, ";")
		part = textproto.TrimString(part)
		if part == "" {
			continue
		}
		name, val, _ := strings.Cut(part, "=")
		cookies = append(cookies, &Cookie{Name: name, Value: val})
	}
	return cookies
}
