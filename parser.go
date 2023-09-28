package cmd

import (
	"unicode"
)

func Fields(src string) (out []string) {
	var (
		it   []rune
		q, r rune
		e    bool
	)

	addc := func(r rune) {
		if r != 0 {
			it = append(it, r)
			e = false
		}
	}

	addo := func() {
		if len(it) > 0 {
			out = append(out, string(it))
			it = it[:0]
		}
	}

	for rs := []rune(src); len(rs) > 0; {
		if r, rs = rs[0], rs[1:]; e { //被转义了
			addc(r)
		} else if r == '"' || r == '\'' || r == '`' {
			if q == 0 { //引号头
				q = r
			} else if q == r { //引号尾
				q = 0
			} else { //其他引号
				addc(r)
			}
		} else if r == '\\' {
			if e || len(rs) == 0 {
				addc(r)
			} else {
				if n := rs[0]; q != 0 {
					if e = n == q; !e {
						addc(r)
					}
				} else {
					if e = n == '"' || n == '\'' || n == '`' || n == '\\' || unicode.IsSpace(n); !e {
						addc(r)
					}
				}
			}
		} else if unicode.IsSpace(r) {
			if q != 0 {
				addc(r)
			} else {
				addo()
			}
		} else {
			addc(r)
		}
	}

	addo()
	return
}
