package postman

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

var (
	ErrVariablesNotSupported = errors.New("Variables are not yet implemented")
	ErrScriptSrcNotSupported = errors.New("External scripts are not implemented")

	ErrScriptUnsupportedType = errors.New("Only text/javascript scripts are supported")
	ErrDurationWrongType     = errors.New("Durations must be numbers or strings")
	ErrTimeWrongType         = errors.New("Times must be numbers or strings")
	ErrMissingHeaderKey      = errors.New("Missing key in request header")
)

type Duration time.Duration

func (d *Duration) UnmarshalJSON(b []byte) error {
	var data interface{}
	if err := json.Unmarshal(b, &data); err != nil {
		return err
	}

	switch v := data.(type) {
	case string:
		num, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			duration, err := time.ParseDuration(v)
			if err != nil {
				return err
			}
			*d = Duration(duration)
			break
		}
		*d = Duration(time.Duration(num) * time.Millisecond)
	case float64:
		*d = Duration(time.Duration(v) * time.Millisecond)
	default:
		return ErrDurationWrongType
	}

	return nil
}

type Time time.Time

func (d *Time) UnmarshalJSON(b []byte) error {
	var data interface{}
	if err := json.Unmarshal(b, &data); err != nil {
		return err
	}

	switch v := data.(type) {
	case string:
		// Why.
		if v == "Invalid Date" {
			*d = Time{}
			break
		}

		t, err := time.Parse("Mon Jan 2 2006 15:04:05 GMT-0700 (MST)", v)
		if err != nil {
			return err
		}
		*d = Time(t)
	case float64:
		*d = Time(time.Unix(int64(v), 0))
	default:
		return ErrTimeWrongType
	}

	return nil
}

type ScriptSrc struct{}

func (ScriptSrc) UnmarshalJSON(b []byte) error {
	return ErrScriptSrcNotSupported
}

type ScriptExec string

func (e *ScriptExec) UnmarshalJSON(b []byte) error {
	var data interface{}
	if err := json.Unmarshal(b, &data); err != nil {
		return err
	}

	switch v := data.(type) {
	case string:
		*e = ScriptExec(v)
	case []interface{}:
		lines := make([]string, 0, len(v))
		for _, val := range v {
			switch line := val.(type) {
			case string:
				lines = append(lines, line)
			default:
				lines = append(lines, fmt.Sprint(line))
			}
		}
		*e = ScriptExec(strings.Join(lines, "\n"))
	}

	return nil
}

type ScriptImpl struct {
	ID   string     `json"id"`
	Type string     `json:"type"`
	Exec ScriptExec `json:"exec"`
	Src  ScriptSrc  `json:"src"`
	Name string     `json:"name"`
}

type Script ScriptImpl

func (s *Script) UnmarshalJSON(b []byte) error {
	var data interface{}
	if err := json.Unmarshal(b, &data); err != nil {
		return err
	}

	switch v := data.(type) {
	case string:
		s.Type = "text/javascript"
		s.Exec = ScriptExec(v)
		s.Name = "inline"
	default:
		var impl ScriptImpl
		if err := json.Unmarshal(b, &impl); err != nil {
			return err
		}

		switch impl.Type {
		case "text/javascript":
		case "":
			impl.Type = "text/javascript"
		default:
			return ErrScriptUnsupportedType
		}

		*s = Script(impl)
	}

	return nil
}

type Event struct {
	Listen   string `json:"listen"`
	Script   Script `json:"script"`
	Disabled bool   `json:"disabled"`
}

type HeaderImpl struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type Header HeaderImpl

func (h *Header) UnmarshalJSON(b []byte) error {
	var impl HeaderImpl
	if err := json.Unmarshal(b, &impl); err != nil {
		return err
	}

	if impl.Key == "" {
		return ErrMissingHeaderKey
	}

	*h = Header(impl)
	return nil
}

type Cookie struct {
	Domain   string   `json:"domain"`
	Expires  Time     `json:"expires"`
	MaxAge   Duration `json:"maxAge"`
	HostOnly bool     `json:"hostOnly"`
	HTTPOnly bool     `json:"httpOnly"`
	Name     string   `json:"name"`
	Path     string   `json:"path"`
	Secure   bool     `json:"secure"`
	Session  bool     `json:"session"`
	Value    string   `json:"value`
	// Not parsing extensions. They are wholly uninteresting to us.
}

type Param struct {
	Key     string `json:"key"`
	Value   string `json:"value"`
	Enabled bool   `json:"enabled"`
}

type RequestImpl struct {
	URL    string   `json:"url"` // TODO: Decompose into net/url.URL structs, handle maps
	Auth   Auth     `json:"auth"`
	Method string   `json:"method"`
	Header []Header `json:"header"` // Docs aren't clear on what a string here means?
	Body   struct {
		Mode       string  `json:"mode"`
		Raw        string  `json:"raw"`
		URLEncoded []Param `json:"urlencoded"`
		FormData   []Param `json:"formdata"`
	} `json:"body"`
}

type Request RequestImpl

func (r *Request) UnmarshalJSON(b []byte) error {
	var data interface{}
	if err := json.Unmarshal(b, &data); err != nil {
		return err
	}

	switch v := data.(type) {
	case string:
		r.URL = v
		r.Method = "GET"
	default:
		var impl RequestImpl
		if err := json.Unmarshal(b, &impl); err != nil {
			return err
		}
		if impl.Method == "" {
			impl.Method = "GET"
		}
		*r = Request(impl)
	}

	return nil
}

type Response struct {
	OriginalRequest Request  `json:"originalRequest"`
	ResponseTime    Duration `json:"responseTime"`
	Header          []Header `json:"header"`
	Cookie          []Cookie `json:"cookie"`
	Body            string   `json:"body"`
	Status          string   `json:"status"`
	Code            int      `json:"code"`
}

// The docs for this are vague and I can't find a UI for it anywhere in the Postman app.
type Variable struct {
}

func (Variable) UnmarshalJSON(b []byte) error {
	return ErrVariablesNotSupported
}

type Auth struct {
	Type string `json:"type"`

	AWSv4 struct {
		AccessKey string `json:"accessKey"`
		SecretKey string `json:"secretKey"`
		Region    string `json:"region"`
		Service   string `json:"service"`
	} `json:"awsv4"`

	Basic struct {
		Username string `json:"username"`
		Password string `json:"password"`
	} `json:"basic"`

	Digest struct {
		Username    string `json:"username"`
		Realm       string `json:"realm"`
		Password    string `json:"password"`
		Nonce       string `json:"nonce"`
		NonceCount  string `json:"nonceCount"`
		Algorithm   string `json:"algorithm"`
		QOP         string `json:"qop"`
		ClientNonce string `json:"clientNonce"`
	} `json:"digest"`

	Hawk struct {
		AuthID     string `json:"authId"`
		AuthKey    string `json:"authKey"`
		Algorithm  string `json:"algorithm"`
		User       string `json:"user"`
		Nonce      string `json:"nonce"`
		ExtraData  string `json:"extraData"`
		AppID      string `json:"appId"`
		Delegation string `json:"delegation"`
	} `json:"hawk"`

	OAuth1 struct {
		ConsumerKey     string `json:"consumerKey"`
		ConsumerSecret  string `json:"consumerSecret"`
		Token           string `json:"token"`
		TokenSecret     string `json:"tokenSecret"`
		SignatureMethod string `json:"signatureMethod"`
		Timestamp       string `json:"timeStamp"`
		Nonce           string `json:"nonce"`
		Version         string `json:"version"`
		Realm           string `json:"realm"`
		EncodeOAuthSign string `json:"encodeOAuthSign"`
	} `json:"oauth1"`

	OAuth2 struct {
		AddTokenTo     string `json:"addTokenTo"`
		CallbackURL    string `json:"callBackUrl"`
		AuthURL        string `json:"authUrl"`
		AccessTokenURL string `json:"accessTokenUrl"`
		ClientID       string `json:"clientId"`
		ClientSecret   string `json:"clientSecret"`
		Scope          string `json:"scope"`

		RequestAccessTokenLocally string `json:"requestAccessTokenLocally"`
	} `json:"oauth2"`
}

type Item struct {
	// Items + Folders
	Name string `json:"name"`

	// Items
	ID       string     `json:"id"`
	Event    []Event    `json:"event"`
	Request  Request    `json:"request"`
	Response []Response `json:"response"`

	// Folders
	Description string `json:"description"`
	Item        []Item `json:"item"`
	Auth        Auth   `json:"auth"`
}

type Information struct {
	Name string `json:"name"`
}

type Collection struct {
	Info     Information `json:"info"`
	Item     []Item      `json:"item"`
	Event    []Event     `json:"event"`
	Variable []Variable  `json:"variable"`
	Auth     Auth        `json:"auth"`
}
