package main

import (
	"fmt"
)

//nolint:gochecknoglobals
var contributors = map[string]bool{
	// core team
	"mstoykov":     true,
	"codebien":     true,
	"olegbespalov": true,
	"oleiade":      true,

	// browser team
	"ankur22":    true,
	"inancgumus": true,
	"ka3de":      true,
}

type change struct {
	RawType string
	Type    PullType
	Number  int
	Title   string
	Body    string
	Author  string
}

func (c change) Format() string {
	if c.isEpic() {
		return c.formatEpic()
	}
	text := fmt.Sprintf("- [#%d](https://github.com/grafana/k6/pull/%d) %s", c.Number, c.Number, c.Title)
	if c.isExternalContributor() {
		text += fmt.Sprintf(" Thanks @%s for the contribution!.", c.Author)
	}
	return text
}

func (c change) formatEpic() string {
	text := fmt.Sprintf("### %s\n\n%s", c.Title, c.Body)
	if c.isExternalContributor() {
		text += fmt.Sprintf("\nThanks @%s for the contribution!.", c.Author)
	}
	return text + "\n"
}

func (c change) isEpic() bool {
	return c.Type == EpicFeature || c.Type == EpicBreaking
}

func (c change) isExternalContributor() bool {
	return contributors[c.Author]
}
