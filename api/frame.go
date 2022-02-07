/*
 *
 * xk6-browser - a browser automation extension for k6
 * Copyright (C) 2021 Load Impact
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as
 * published by the Free Software Foundation, either version 3 of the
 * License, or (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 *
 */

package api

import "github.com/dop251/goja"

// Frame is the interface of a CDP target frame.
type Frame interface {
	AddScriptTag(opts goja.Value)
	AddStyleTag(opts goja.Value)
	Check(selector string, opts goja.Value)
	ChildFrames() []Frame
	Click(selector string, opts goja.Value)
	Content() string
	Dblclick(selector string, opts goja.Value)
	DispatchEvent(selector string, typ string, eventInit goja.Value, opts goja.Value)
	Evaluate(pageFunc goja.Value, args ...goja.Value) interface{}
	EvaluateHandle(pageFunc goja.Value, args ...goja.Value) JSHandle
	Fill(selector string, value string, opts goja.Value)
	Focus(selector string, opts goja.Value)
	FrameElement() ElementHandle
	GetAttribute(selector string, name string, opts goja.Value) goja.Value
	Goto(url string, opts goja.Value) Response
	Hover(selector string, opts goja.Value)
	InnerHTML(selector string, opts goja.Value) string
	InnerText(selector string, opts goja.Value) string
	InputValue(selector string, opts goja.Value) string
	IsChecked(selector string, opts goja.Value) bool
	IsDetached() bool
	IsDisabled(selector string, opts goja.Value) bool
	IsEditable(selector string, opts goja.Value) bool
	IsEnabled(selector string, opts goja.Value) bool
	IsHidden(selector string, opts goja.Value) bool
	IsVisible(selector string, opts goja.Value) bool
	ID() string
	LoaderID() string
	Name() string
	Query(selector string) ElementHandle
	QueryAll(selector string) []ElementHandle
	Page() Page
	ParentFrame() Frame
	Press(selector string, key string, opts goja.Value)
	SelectOption(selector string, values goja.Value, opts goja.Value) []string
	SetContent(html string, opts goja.Value)
	SetInputFiles(selector string, files goja.Value, opts goja.Value)
	Tap(selector string, opts goja.Value)
	TextContent(selector string, opts goja.Value) string
	Title() string
	Type(selector string, text string, opts goja.Value)
	Uncheck(selector string, opts goja.Value)
	URL() string
	WaitForFunction(pageFunc goja.Value, opts goja.Value, args ...goja.Value) JSHandle
	WaitForLoadState(state string, opts goja.Value)
	WaitForNavigation(opts goja.Value) Response
	WaitForSelector(selector string, opts goja.Value) ElementHandle
	WaitForTimeout(timeout int64)
}
