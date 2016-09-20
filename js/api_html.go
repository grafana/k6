package js

import (
	"github.com/PuerkitoBio/goquery"
	"strings"
)

func (a JSAPI) HTMLParse(src string) *goquery.Selection {
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(src))
	if err != nil {
		throw(a.vu.vm, err)
	}
	return doc.Selection
}
