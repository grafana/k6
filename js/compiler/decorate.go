package compiler

import (
	"fmt"
	"strings"
)

type classDecorate struct {
	className string
	code      string
	endIndex  int
}

// decorate
//
//	@Description: 更改源码，调整修饰器代码位置
//	@receiver c
//	@param code
//	@param start
//	@return string
func newClassDecorate(jsCode string, startIndex int) *classDecorate {
	// lxd
	this := &classDecorate{code: jsCode, endIndex: -1}
	if startIndex >= len(jsCode) {
		return this
	}
	headCode := ""
	code := jsCode
	if startIndex > 0 {
		headCode = jsCode[:startIndex]
		code = jsCode[startIndex:]
	}

	// 1. find the class name
	i := strings.Index(code, " = /** @class */")
	if i == -1 {
		return this
	}
	n := strings.LastIndex(code[0:i], "var ")
	if n == -1 {
		return this
	}
	className := code[n+len("var ") : i]
	this.className = className
	hasClass := false
	start := strings.Index(code, fmt.Sprintf("%s = __decorate([", className))
	classEnd := 0
	if start > -1 {
		endStr := fmt.Sprintf(`], %s);`, className)
		end := strings.Index(code, endStr)

		if end > -1 {
			end = end + len(endStr)
			newCode := code[start:end]
			newCode = strings.ReplaceAll(newCode, fmt.Sprintf("%s = ", className), "")
			newCode = strings.ReplaceAll(newCode, fmt.Sprintf("], %s);", className), "], this);")
			code = code[0:start] + newCode + "\n    }\n" + code[end:]
			this.endIndex = end
			hasClass = true
			classEnd = end
		}
	}

	// 2. find the method decorator
	methodEnd := 0
	start = strings.Index(code[:classEnd], "__decorate([")
	if start > -1 {
		endChar := ", void 0);"
		end := strings.LastIndex(code, endChar)
		if end > -1 {
			end = end + len(endChar)
			decorate := code[start:end]
			if className != "" {
				decorate = strings.ReplaceAll(decorate, className+".prototype", "this")
				st := -1
				for i := start; i >= 0; i-- {
					if code[i] == '}' {
						st = i
						break
					}
				}
				decorate = "\n     // lxd \n    " + decorate + "\n"
				if !hasClass {
					decorate += "}"
				}
				code = code[:st] + decorate + code[end:]
				methodEnd = end
			}
		}
	}
	this.endIndex = classEnd
	if classEnd < methodEnd {
		this.endIndex = methodEnd
	}
	this.code = headCode + code
	return this
}

func (d *classDecorate) Code() string {
	return d.code
}

type classCode struct {
	start     int
	end       int
	className string
	code      string
}

func newClassCode(code string, start int) (*classCode, bool) {
	if start > 0 {
		code = code[start:]
	}
	c := strings.Index(code, " = /** @class */")
	if c == -1 {
		return nil, false
	}
	n := strings.LastIndex(code[:c], "var ")
	if n == -1 {
		return nil, false
	}
	className := strings.ReplaceAll(code[n:c], "var ", "")
	endStr := fmt.Sprintf("return %s", className)
	end := strings.Index(code[n:], endStr)
	if end == -1 {
		return nil, false
	}

	end = n + end + len(endStr)
	return &classCode{
		className: className,
		code:      code[n:end],
		start:     start + n,
		end:       start + end,
	}, true
}

const initCode = "\n    if (this.__init) { this.__init(); } \n    }"

func (c *classCode) replace() {
	className := c.className

	// 查找 "__decorate(["  ......... ", void 0);" 代码段
	start := "__decorate(["
	end := ", void 0);"
	method := between(c.code, start, end)
	if method != nil {
		method.code = strings.ReplaceAll(method.code, className+".prototype", "this")
	}

	// 查找 "HumanFeign = __decorate(["  .........  "], HumanFeign);" 代码段
	start = fmt.Sprintf("%s = __decorate([", className)
	end = fmt.Sprintf("], %s);", className)
	class := between(c.code, start, end)
	if class != nil {
		class.code = strings.ReplaceAll(class.code, fmt.Sprintf("%s = ", className), "")
		class.code = strings.ReplaceAll(class.code, fmt.Sprintf("], %s);", className), "], this);")
	}

	// 删除掉 func HumanFeign(){ }中最后的括号
	begin := func(code string) string {
		i := strings.LastIndex(code, "}")
		if i == -1 {
			return code
		}
		return code[:i] + "\n    "
	}
	// 将 decorate 代码拼接到 func HumanFeign(){} 构造函数中。
	var n, m int
	if method != nil && class != nil {
		n = method.start
		m = class.end
		c.code = begin(c.code[:n]) + class.code + "\n    " + method.code + initCode + c.code[m:]
	} else if method != nil {
		n = method.start
		m = method.end
		c.code = begin(c.code[:n]) + method.code + initCode + c.code[m:]
	} else if class != nil {
		n = class.start
		m = class.end
		c.code = begin(c.code[:n]) + class.code + initCode + c.code[m:]
	}

}

type betweenCode struct {
	code  string
	start int
	end   int
}

func between(str, start, end string) *betweenCode {
	n := strings.Index(str, start)
	if n == -1 {
		return nil
	}
	str = str[n:]
	m := strings.LastIndex(str, end)
	if m == -1 {
		return nil
	}
	return &betweenCode{code: str[:m+len(end)], start: n, end: n + m + len(end)}
}
