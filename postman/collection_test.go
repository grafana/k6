package postman

import (
	"encoding/json"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func TestUnmarshalDuration(t *testing.T) {
	var d Duration
	assert.NoError(t, json.Unmarshal([]byte(`12345`), &d))
	assert.Equal(t, Duration(12345*time.Millisecond), d)
}

func TestUnmarshalDurationString(t *testing.T) {
	var d Duration
	assert.NoError(t, json.Unmarshal([]byte(`"12345"`), &d))
	assert.Equal(t, Duration(12345*time.Millisecond), d)
}

func TestUnmarshalDurationStringUnit(t *testing.T) {
	var d Duration
	assert.NoError(t, json.Unmarshal([]byte(`"12345s"`), &d))
	assert.Equal(t, Duration(12345*time.Second), d)
}

func TestUnmarshalDurationStringInvalid(t *testing.T) {
	var d Duration
	assert.Error(t, json.Unmarshal([]byte(`{}`), &d))
}

func TestUnmarshalDurationStringInvalidJSON(t *testing.T) {
	var d Duration
	assert.Error(t, json.Unmarshal([]byte(`e`), &d))
}

func TestUnmarshalTime(t *testing.T) {
	var tm Time
	assert.NoError(t, json.Unmarshal([]byte(`12345`), &tm))
	assert.Equal(t, int64(12345), time.Time(tm).Unix())
}

func TestUnmarshalTimeString(t *testing.T) {
	var tm Time
	assert.NoError(t, json.Unmarshal([]byte(`"Fri Apr 15 2016 12:54:28 GMT+0200 (CEST)"`), &tm))
	assert.Equal(t, int64(1460717668), time.Time(tm).Unix())
}

func TestUnmarshalTimeInvalid(t *testing.T) {
	var tm Time
	assert.Error(t, json.Unmarshal([]byte(`{}`), &tm))
}

func TestUnmarshalTimeInvalidJSON(t *testing.T) {
	var tm Time
	assert.Error(t, json.Unmarshal([]byte(`e`), &tm))
}

func TestUnmarshalInfo(t *testing.T) {
	j := []byte(`{
		"info": { "name": "Name" }
	}`)

	c := Collection{}
	assert.NoError(t, json.Unmarshal(j, &c))
	assert.Equal(t, "Name", c.Info.Name)
}

func TestUnmarshalItem(t *testing.T) {
	j := []byte(`{
		"item": [
			{ "id": "1", "name": "A" },
			{ "id": "2", "name": "B" }
		]
	}`)

	c := Collection{}
	assert.NoError(t, json.Unmarshal(j, &c))
	assert.Len(t, c.Item, 2)
	assert.Equal(t, "1", c.Item[0].ID)
	assert.Equal(t, "A", c.Item[0].Name)
	assert.Equal(t, "2", c.Item[1].ID)
	assert.Equal(t, "B", c.Item[1].Name)
}

func TestUnmarshalItemNested(t *testing.T) {
	j := []byte(`{
		"item": [{
			"name": "Folder",
			"description": "Lorem ipsum",
			"item": [
				{ "id": "1", "name": "A" },
				{ "id": "2", "name": "B" }
			]
		}]
	}`)

	c := Collection{}
	assert.NoError(t, json.Unmarshal(j, &c))
	assert.Len(t, c.Item, 1)
	assert.Equal(t, "Folder", c.Item[0].Name)
	assert.Equal(t, "Lorem ipsum", c.Item[0].Description)
	assert.Len(t, c.Item[0].Item, 2)
	assert.Equal(t, "1", c.Item[0].Item[0].ID)
	assert.Equal(t, "A", c.Item[0].Item[0].Name)
	assert.Equal(t, "2", c.Item[0].Item[1].ID)
	assert.Equal(t, "B", c.Item[0].Item[1].Name)
}

func TestUnmarshalItemRequest(t *testing.T) {
	j := []byte(`{
		"item": [{
			"request": {
				"url": "http://example.com/",
				"method": "POST",
				"header": [{"key": "Content-Type", "value": "text/plain"}],
				"body": { "mode": "raw", "raw": "lorem ipsum" }
			}
		}]
	}`)

	c := Collection{}
	assert.NoError(t, json.Unmarshal(j, &c))
	assert.Len(t, c.Item, 1)
	assert.Equal(t, "http://example.com/", c.Item[0].Request.URL)
	assert.Equal(t, "POST", c.Item[0].Request.Method)
	assert.Len(t, c.Item[0].Request.Header, 1)
	assert.Equal(t, "Content-Type", c.Item[0].Request.Header[0].Key)
	assert.Equal(t, "text/plain", c.Item[0].Request.Header[0].Value)
	assert.Equal(t, "raw", c.Item[0].Request.Body.Mode)
	assert.Equal(t, "lorem ipsum", c.Item[0].Request.Body.Raw)
}

func TestUnmarshalItemRequestImplicitMethod(t *testing.T) {
	j := []byte(`{
		"item": [{
			"request": {
				"url": "http://example.com/"
			}
		}]
	}`)

	c := Collection{}
	assert.NoError(t, json.Unmarshal(j, &c))
	assert.Len(t, c.Item, 1)
	assert.Equal(t, "http://example.com/", c.Item[0].Request.URL)
	assert.Equal(t, "GET", c.Item[0].Request.Method)
}

func TestUnmarshalItemRequestString(t *testing.T) {
	j := []byte(`{
		"item": [{
			"request": "http://example.com/"
		}]
	}`)

	c := Collection{}
	assert.NoError(t, json.Unmarshal(j, &c))
	assert.Len(t, c.Item, 1)
	assert.Equal(t, "http://example.com/", c.Item[0].Request.URL)
	assert.Equal(t, "GET", c.Item[0].Request.Method)
	assert.Len(t, c.Item[0].Request.Header, 0)
}

func TestUnmarshalItemRequestMissingHeaderKey(t *testing.T) {
	j := []byte(`{
		"item": [{
			"request": {
				"url": "http://example.com/",
				"method": "POST",
				"header": [{"value": "text/plain"}]
			}
		}]
	}`)

	c := Collection{}
	assert.Equal(t, ErrMissingHeaderKey, json.Unmarshal(j, &c))
}

func TestUnmarshalItemResponse(t *testing.T) {
	j := []byte(`{
		"item": [{
			"response": [{
				"originalRequest": "http://example.com/",
				"responseTime": 100,
				"header": [{"key": "Content-Type", "value": "text/plain"}],
				"body": "lorem ipsum",
				"status": "200 OK",
				"code": 200
			}]
		}]
	}`)

	c := Collection{}
	assert.NoError(t, json.Unmarshal(j, &c))
	assert.Len(t, c.Item, 1)
	assert.Len(t, c.Item[0].Response, 1)
	assert.Equal(t, "http://example.com/", c.Item[0].Response[0].OriginalRequest.URL)
	assert.Equal(t, "GET", c.Item[0].Response[0].OriginalRequest.Method)
	assert.Equal(t, 100*time.Millisecond, time.Duration(c.Item[0].Response[0].ResponseTime))
	assert.Len(t, c.Item[0].Response[0].Header, 1)
	assert.Equal(t, "Content-Type", c.Item[0].Response[0].Header[0].Key)
	assert.Equal(t, "text/plain", c.Item[0].Response[0].Header[0].Value)
	assert.Equal(t, "lorem ipsum", c.Item[0].Response[0].Body)
	assert.Equal(t, "200 OK", c.Item[0].Response[0].Status)
	assert.Equal(t, 200, c.Item[0].Response[0].Code)
}

func TestUnmarshalItemResponseCookie(t *testing.T) {
	j := []byte(`{
		"item": [{
			"response": [{
				"originalRequest": "http://example.com/",
				"cookie": [{
					"domain": "example.com",
					"expires": 1,
					"maxAge": 123,
					"hostOnly": true,
					"httpOnly": true,
					"name": "name",
					"path": "/",
					"secure": true,
					"session": true,
					"value": "value"
				}]
			}]
		}]
	}`)

	c := Collection{}
	assert.NoError(t, json.Unmarshal(j, &c))
	assert.Len(t, c.Item, 1)
	assert.Len(t, c.Item[0].Response, 1)
	assert.Len(t, c.Item[0].Response[0].Cookie, 1)
	assert.Equal(t, "example.com", c.Item[0].Response[0].Cookie[0].Domain)
	assert.Equal(t, int64(1), time.Time(c.Item[0].Response[0].Cookie[0].Expires).Unix())
	assert.Equal(t, 123*time.Millisecond, time.Duration(c.Item[0].Response[0].Cookie[0].MaxAge))
	assert.Equal(t, true, c.Item[0].Response[0].Cookie[0].HostOnly)
	assert.Equal(t, true, c.Item[0].Response[0].Cookie[0].HTTPOnly)
	assert.Equal(t, "name", c.Item[0].Response[0].Cookie[0].Name)
	assert.Equal(t, "/", c.Item[0].Response[0].Cookie[0].Path)
	assert.Equal(t, true, c.Item[0].Response[0].Cookie[0].Secure)
	assert.Equal(t, true, c.Item[0].Response[0].Cookie[0].Session)
	assert.Equal(t, "value", c.Item[0].Response[0].Cookie[0].Value)
}

func TestUmmarshalEvent(t *testing.T) {
	j := []byte(`{
		"event": [{
			"listen": "test",
			"script": {
				"id": "script1",
				"type": "text/javascript",
				"exec": "var v = 1 + 1;\nconsole.log(v);",
				"name": "script1.js"
			}
		}]
	}`)

	c := Collection{}
	assert.NoError(t, json.Unmarshal(j, &c))
	assert.Len(t, c.Event, 1)
	assert.Equal(t, "test", c.Event[0].Listen)
	assert.Equal(t, "script1", c.Event[0].Script.ID)
	assert.Equal(t, "text/javascript", c.Event[0].Script.Type)
	assert.Equal(t, ScriptExec("var v = 1 + 1;\nconsole.log(v);"), c.Event[0].Script.Exec)
	assert.Equal(t, "script1.js", c.Event[0].Script.Name)
	assert.Equal(t, false, c.Event[0].Disabled)
}

func TestUmmarshalEventArrayExec(t *testing.T) {
	j := []byte(`{
		"event": [{
			"listen": "test",
			"script": {
				"id": "script1",
				"type": "text/javascript",
				"exec": [
					"var v = 1 + 1;",
					"console.log(v);"
				],
				"name": "script1.js"
			}
		}]
	}`)

	c := Collection{}
	assert.NoError(t, json.Unmarshal(j, &c))
	assert.Len(t, c.Event, 1)
	assert.Equal(t, "test", c.Event[0].Listen)
	assert.Equal(t, "script1", c.Event[0].Script.ID)
	assert.Equal(t, "text/javascript", c.Event[0].Script.Type)
	assert.Equal(t, ScriptExec("var v = 1 + 1;\nconsole.log(v);"), c.Event[0].Script.Exec)
	assert.Equal(t, "script1.js", c.Event[0].Script.Name)
	assert.Equal(t, false, c.Event[0].Disabled)
}

func TestUmmarshalEventScriptImplicitType(t *testing.T) {
	j := []byte(`{
		"event": [{
			"listen": "test",
			"script": {
				"exec": "var v = 1 + 1;\nconsole.log(v);"
			}
		}]
	}`)

	c := Collection{}
	assert.NoError(t, json.Unmarshal(j, &c))
	assert.Len(t, c.Event, 1)
	assert.Equal(t, "test", c.Event[0].Listen)
	assert.Equal(t, "text/javascript", c.Event[0].Script.Type)
	assert.Equal(t, ScriptExec("var v = 1 + 1;\nconsole.log(v);"), c.Event[0].Script.Exec)
}

func TestUmmarshalEventWrongScriptType(t *testing.T) {
	j := []byte(`{
		"event": [{
			"listen": "test",
			"script": {
				"type": "text/vbscript",
				"exec": "/* I don't actually know VBScript lol */"
			}
		}]
	}`)

	c := Collection{}
	assert.Equal(t, ErrScriptUnsupportedType, json.Unmarshal(j, &c))
}

func TestUmmarshalEventScriptString(t *testing.T) {
	j := []byte(`{
		"event": [{
			"listen": "test",
			"script": "var v = 1 + 1;\nconsole.log(v);"
		}]
	}`)

	c := Collection{}
	assert.NoError(t, json.Unmarshal(j, &c))
	assert.Len(t, c.Event, 1)
	assert.Equal(t, "test", c.Event[0].Listen)
	assert.Equal(t, "text/javascript", c.Event[0].Script.Type)
	assert.Equal(t, ScriptExec("var v = 1 + 1;\nconsole.log(v);"), c.Event[0].Script.Exec)
}

func TestUmmarshalEventScriptInvalid(t *testing.T) {
	j := []byte(`{
		"event": [{
			"listen": "test",
			"script": 12345
		}]
	}`)

	c := Collection{}
	assert.Error(t, json.Unmarshal(j, &c))
}

func TestUnmarshalVariableNotImplemented(t *testing.T) {
	j := []byte(`{ "variable": [{}] }`)
	c := Collection{}
	assert.Equal(t, ErrVariablesNotSupported, json.Unmarshal(j, &c))
}
