package html

import (
	"testing"

	"github.com/stretchr/testify/require"
)

const testGenElems = `<html><body>
	<a id="a1" download="file:///path/name" referrerpolicy="no-referrer" rel="open" href="http://test.url" target="__blank" type="text/html" accesskey="w" hreflang="es"></a>
	<a id="a2"></a>
	<a id="a3" href="relpath"></a>
	<a id="a4" href="/abspath"></a>
	<a id="a5" href="?a=yes-a&b=yes-b"></a>
	<a id="a6" href="#testfrag"></a>
	<a id="a7" href="../prtpath"></a>
	<a id="a8" href=""></a>
	<audio id="audio1" autoplay controls loop muted src="foo.wav" crossorigin="anonymous" mediagroup="testgroup"></audio>
	<audio id="audio2"></audio>
	<audio id="audio3" src=""></audio>
	<base id="base1" href="foo.html" target="__any"></base>
	<base id="base2"></base>
	<base id="base3" href="" target="__any"></base>
	<button id="btn1" accesskey="e" target="__any" autofocus disabled type="button"></button>
	<button id="btn2"></button>
	<button id="btn3" type="invalid_uses_default"></button> <button id="btn3" type="invalid_uses_default"></button>
	<ul><li><data id="data1" value="121"></data></li><li><data id="data2"></data></li></ul>
	<embed id="embed1" type="video/avi" src="movie.avi" width="640" height="480">
	<fieldset id="fset1" disabled name="fset1_name"></fieldset>
	<fieldset id="fset2"></fieldset>
	<form id="form1" name="form1_name" target="__self" enctype="text/plain" action="submit_url" accept-charset="ISO-8859-1" autocomplete="off" novalidate></form>
	<form id="form2"></form>
	<form id="form3" action=""></form>
	<iframe id="iframe1" allowfullscreen referrerpolicy="no-referrer" name="frame_name" width="640" height="480" src="testframe.html"></iframe>
	<iframe id="iframe2" referrerpolicy="use-default-when-invalid"></iframe>
	<iframe id="iframe3" src=""></iframe>
	<img id="img1" src="test.png" sizes="100vw,50vw" srcset="large.jpg 1024w,medium.jpg 640w" alt="alt text" crossorigin="anonymous" height="50" width="100" ismap name="img_name" usemap="#map_name" referrerpolicy="origin"/>
	<img id="img2" crossorigin="use-credentials" referrerpolicy="use-default-when-invalid"/>
	<img id="img3" src=""/>
	<input id="input1" name="input1_name" disabled autofocus required value="input1-val" type="button"/>
	<input id="input2"/>
	<input id="input3" type="checkbox" checked multiple/>
	<input id="input4" type="checkbox"/>
	<input id="input5" type="image" alt="input_img" src="input.png" width="80" height="40"/>
	<input id="input5b" type="image" />
	<input id="input5c" type="image" src=""/>
	<input id="input6" type="file" accept=".jpg,.png"/>
	<input id="input7" type="text" autocomplete="off" maxlength="10" size="5" pattern="..." placeholder="help text" readonly min="2017-01-01" max="2017-12-12" dirname="input7.dir" accesskey="s" step="0.1"/>
	<keygen id="kg1" autofocus challenge="cx1" disabled keytype="DSA" name="kg1_name"/>
	<keygen id="kg2"/>
	<label id="label1" for="input1_name"/>
	<legend id="legend1" accesskey="l"/>
	<li id="li1" type="disc"></li> <li id="li2" value="10" type=""></li>
	<link id="link1" crossorigin="use-credentials" referrerpolicy="no-referrer" href="test.css" hreflang="pl" media="print" rel="alternate author" target="__self" type="stylesheet"/>
	<link id="link2"/>
	<link id="link3" href=""/>
	<map id="map1" name="map1_name"></map>
	<meta id="meta1" name="author" content="author name" />
	<meta id="meta2" http-equiv="refresh" content="1;www.test.com" />
	<meter id="meter1" min="90" max="110" low="95" high="105" optimum="100"/>
	<ins id="ins1" cite="cite.html" datetime="2017-01-01"/>
	<object id="object1" name="obj1_name" data="test.png" type="image/png" width="150" height="75" tabindex="6" typemustmatch usemap="#map1_name"/>
	<object id="object2"/>
	<object id="object3" data=""/>
	<ol id="ol1" reversed start="1" type="a"></ol> <ol id="ol2"></ol>
	<optgroup id="optgroup1" disabled label="optlabel"></optgroup>
	<optgroup id="optgroup2"></optgroup>
	<option id="opt1" selected/><option id="opt2" />
	<output id="out1" for="input1" name="out1_name"/>
	<param id="par1" name="param1_name" value="param1_val"/>
	<pre id="pre1" name="pre1_name" value="pre1_val"/>
	<quote id="quote1" cite="http://cite.com/url"/>
	<script id="script1" crossorigin="use-credentials" type="text/javascript" src="script.js" charset="ISO-8859-1" defer async nomodule></script>
	<script id="script2"></script>
	<script id="script3" src=""></script>
	<select id="select1" name="sel1_name" autofocus disabled multiple required></select>
	<select id="select2"></select>
	<source id="src1" keysystem="keysys" media="(min-width: 600px)" sizes="100vw,50vw" srcset="large.jpg 1024w,medium.jpg 640w" src="test.png" type="image/png"></source>
	<source id="src2"></source>
	<source id="src3" src=""></source>
	<style id="style1" media="print"></style>
	<table id="table1" sortable><tr><td id="td1" colspan="2" rowspan="3" headers="th1"></th><th id="th1" abbr="hdr" scope="row" sorted>Header</th><th id="th2"></th></tr></table>
	<table id="table2"></table>
	<textarea id="txtarea1" value="init_txt" placeholder="display_txt" rows="10" cols="12" maxlength="128" accesskey="k" tabIndex="4" readonly required autocomplete="off" autocapitalize="words" wrap="hard"></textarea>
	<textarea id="txtarea2"></textarea>
	<time id="time1" datetime="2017-01-01"/>
	<track id="track1" kind="metadata" src="foo.en.vtt" srclang="en" label="English"></track>
	<track id="track2" src="foo.sv.vtt" srclang="sv" label="Svenska"></track>
	<track id="track3"></track>
	<track id="track4" src=""></track>
	<ul id="ul1" type="circle"/>
	`

func TestGenElementsTestTextProperties(t *testing.T) {
	t.Parallel()
	rt := getTestRuntimeWithDoc(t, testGenElems)

	textTests := []struct {
		id       string
		property string
		data     string
	}{
		{"a1", "download", "file:///path/name"},
		{"a1", "referrerPolicy", "no-referrer"},
		{"a1", "href", "http://test.url"},
		{"a1", "target", "__blank"},
		{"a1", "type", "text/html"},
		{"a1", "accessKey", "w"},
		{"a1", "hrefLang", "es"},
		{"a1", "toString", "http://test.url"},
		{"a2", "referrerPolicy", ""},
		{"a2", "accessKey", ""},
		{"audio1", "src", "foo.wav"},
		{"audio1", "crossOrigin", "anonymous"},
		{"audio1", "currentSrc", "foo.wav"},
		{"audio1", "mediaGroup", "testgroup"},
		{"base1", "href", "foo.html"},
		{"base1", "target", "__any"},
		{"btn1", "accessKey", "e"},
		{"btn1", "type", "button"},
		{"btn2", "type", "submit"},
		{"btn3", "type", "submit"},
		{"data1", "value", "121"},
		{"data2", "value", ""},
		{"embed1", "type", "video/avi"},
		{"embed1", "src", "movie.avi"},
		{"embed1", "width", "640"},
		{"embed1", "height", "480"},
		{"fset1", "name", "fset1_name"},
		{"form1", "target", "__self"},
		{"form1", "action", "submit_url"},
		{"form1", "enctype", "text/plain"},
		{"form1", "encoding", "text/plain"},
		{"form1", "acceptCharset", "ISO-8859-1"},
		{"form1", "target", "__self"},
		{"form1", "autocomplete", "off"},
		{"form2", "enctype", "application/x-www-form-urlencoded"},
		{"form2", "autocomplete", "on"},
		{"iframe1", "referrerPolicy", "no-referrer"},
		{"iframe2", "referrerPolicy", ""},
		{"iframe3", "referrerPolicy", ""},
		{"iframe1", "width", "640"},
		{"iframe1", "height", "480"},
		{"iframe1", "name", "frame_name"},
		{"iframe1", "src", "testframe.html"},
		{"img1", "src", "test.png"},
		{"img1", "currentSrc", "test.png"},
		{"img1", "sizes", "100vw,50vw"},
		{"img1", "srcset", "large.jpg 1024w,medium.jpg 640w"},
		{"img1", "alt", "alt text"},
		{"img1", "crossOrigin", "anonymous"},
		{"img1", "name", "img_name"},
		{"img1", "useMap", "#map_name"},
		{"img1", "referrerPolicy", "origin"},
		{"img2", "crossOrigin", "use-credentials"},
		{"img2", "referrerPolicy", ""},
		{"img3", "referrerPolicy", ""},
		{"input1", "name", "input1_name"},
		{"input1", "type", "button"},
		{"input1", "value", "input1-val"},
		{"input1", "defaultValue", "input1-val"},
		{"input2", "type", "text"},
		{"input2", "value", ""},
		{"input5", "alt", "input_img"},
		{"input5", "src", "input.png"},
		{"input5", "width", "80"},
		{"input5", "height", "40"},
		{"input6", "accept", ".jpg,.png"},
		{"input7", "autocomplete", "off"},
		{"input7", "pattern", "..."},
		{"input7", "placeholder", "help text"},
		{"input7", "min", "2017-01-01"},
		{"input7", "max", "2017-12-12"},
		{"input7", "dirName", "input7.dir"},
		{"input7", "accessKey", "s"},
		{"input7", "step", "0.1"},
		{"kg1", "challenge", "cx1"},
		{"kg1", "keytype", "DSA"},
		{"kg1", "name", "kg1_name"},
		{"kg2", "challenge", ""},
		{"kg2", "keytype", "RSA"},
		{"kg2", "type", "keygen"},
		{"label1", "htmlFor", "input1_name"},
		{"legend1", "accessKey", "l"},
		{"li1", "type", "disc"},
		{"li2", "type", ""},
		{"link1", "crossOrigin", "use-credentials"},
		{"link1", "referrerPolicy", "no-referrer"},
		{"link1", "href", "test.css"},
		{"link1", "hreflang", "pl"},
		{"link1", "media", "print"},
		{"link1", "rel", "alternate author"},
		{"link1", "target", "__self"},
		{"link1", "type", "stylesheet"},
		{"link2", "referrerPolicy", ""},
		{"map1", "name", "map1_name"},
		{"meta1", "name", "author"},
		{"meta1", "content", "author name"},
		{"meta2", "httpEquiv", "refresh"},
		{"meta2", "content", "1;www.test.com"},
		{"meta2", "content", "1;www.test.com"},
		{"ins1", "cite", "cite.html"},
		{"ins1", "datetime", "2017-01-01"},
		{"object1", "data", "test.png"},
		{"object1", "type", "image/png"},
		{"object1", "name", "obj1_name"},
		{"object1", "width", "150"},
		{"object1", "height", "75"},
		{"object1", "useMap", "#map1_name"},
		{"ol1", "type", "a"},
		{"optgroup1", "label", "optlabel"},
		{"out1", "htmlFor", "input1"},
		{"out1", "name", "out1_name"},
		{"out1", "type", "output"},
		{"par1", "name", "param1_name"},
		{"par1", "value", "param1_val"},
		{"pre1", "name", "pre1_name"},
		{"pre1", "value", "pre1_val"},
		{"quote1", "cite", "http://cite.com/url"},
		{"script1", "crossOrigin", "use-credentials"},
		{"script1", "type", "text/javascript"},
		{"script1", "src", "script.js"},
		{"script1", "charset", "ISO-8859-1"},
		{"select1", "name", "sel1_name"},
		{"src1", "keySystem", "keysys"},
		{"src1", "media", "(min-width: 600px)"},
		{"src1", "sizes", "100vw,50vw"},
		{"src1", "srcset", "large.jpg 1024w,medium.jpg 640w"},
		{"src1", "src", "test.png"},
		{"src1", "type", "image/png"},
		{"td1", "headers", "th1"},
		{"th1", "abbr", "hdr"},
		{"th1", "scope", "row"},
		{"txtarea1", "accessKey", "k"},
		{"txtarea1", "autocomplete", "off"},
		{"txtarea1", "autocapitalize", "words"},
		{"txtarea1", "wrap", "hard"},
		{"txtarea2", "autocomplete", "on"},
		{"txtarea2", "autocapitalize", "sentences"},
		{"txtarea2", "wrap", "soft"},
		{"track1", "kind", "metadata"},
		{"track1", "src", "foo.en.vtt"},
		{"track1", "label", "English"},
		{"track1", "srclang", "en"},
		{"track2", "kind", "subtitle"},
		{"track2", "src", "foo.sv.vtt"},
		{"track2", "srclang", "sv"},
		{"track2", "label", "Svenska"},
		{"time1", "datetime", "2017-01-01"},
		{"ul1", "type", "circle"},
	}
	for _, test := range textTests {
		v, err := rt.RunString(`doc.find("#` + test.id + `").get(0).` + test.property + `()`)
		if err != nil {
			t.Errorf("Error for property name '%s' on element id '#%s':\n%+v ", test.id, test.property, err)
		} else if v.Export() != test.data {
			t.Errorf("Expected '%s' for property name '%s' element id '#%s'. Got '%s'", test.data, test.property, test.id, v.String())
		}
	}
}

func TestGenElementsBoolProperties(t *testing.T) {
	t.Parallel()
	rt := getTestRuntimeWithDoc(t, testGenElems)

	boolTests := []struct {
		idTrue   string
		idFalse  string
		property string
	}{
		{"audio1", "audio2", "autoplay"},
		{"audio1", "audio2", "controls"},
		{"audio1", "audio2", "loop"},
		{"audio1", "audio2", "muted"},
		{"audio1", "audio2", "defaultMuted"},
		{"btn1", "btn2", "autofocus"},
		{"btn1", "btn2", "disabled"},
		{"fset1", "fset2", "disabled"},
		{"form1", "form2", "noValidate"},
		{"iframe1", "iframe2", "allowfullscreen"},
		{"img1", "img2", "isMap"},
		{"input1", "input2", "disabled"},
		{"input1", "input2", "autofocus"},
		{"input1", "input2", "required"},
		{"input3", "input4", "checked"},
		{"input3", "input4", "defaultChecked"},
		{"input7", "input1", "readonly"},
		{"input3", "input4", "multiple"},
		{"kg1", "kg2", "autofocus"},
		{"kg1", "kg2", "disabled"},
		{"object1", "object2", "typeMustMatch"},
		{"ol1", "ol2", "reversed"},
		{"optgroup1", "optgroup2", "disabled"},
		{"opt1", "opt2", "selected"},
		{"opt1", "opt2", "defaultSelected"},
		{"script1", "script2", "async"},
		{"script1", "script2", "defer"},
		{"script1", "script2", "noModule"},
		{"select1", "select2", "autofocus"},
		{"select1", "select2", "disabled"},
		{"select1", "select2", "multiple"},
		{"select1", "select2", "required"},
		{"table1", "table2", "sortable"},

		{"th1", "th2", "sorted"},

		{"txtarea1", "txtarea2", "readOnly"},
		{"txtarea1", "txtarea2", "required"},
	}

	for _, test := range boolTests {
		vT, errT := rt.RunString(`doc.find("#` + test.idTrue + `").get(0).` + test.property + `()`)
		if errT != nil {
			t.Errorf("Error for property name '%s' on element id '#%s':\n%+v", test.property, test.idTrue, errT)
		} else if vT.Export() != true {
			t.Errorf("Expected true for property name '%s' on element id '#%s'", test.property, test.idTrue)
		}

		vF, errF := rt.RunString(`doc.find("#` + test.idFalse + `").get(0).` + test.property + `()`)
		if errF != nil {
			t.Errorf("Error for property name '%s' on element id '#%s':\n%+v", test.property, test.idFalse, errF)
		} else if vF.Export() != false {
			t.Errorf("Expected false for property name '%s' on element id '#%s'", test.property, test.idFalse)
		}
	}
}

func TestGenElementsIntProperties(t *testing.T) {
	t.Parallel()
	rt := getTestRuntimeWithDoc(t, testGenElems)

	intTests := []struct {
		id       string
		property string
		data     int
	}{
		{"img1", "width", 100},
		{"img1", "height", 50},
		{"input7", "maxLength", 10},
		{"input7", "size", 5},
		{"li1", "value", 0},
		{"li2", "value", 10},
		{"meter1", "min", 90},
		{"meter1", "max", 110},
		{"meter1", "low", 95},
		{"meter1", "high", 105},
		{"meter1", "optimum", 100},
		{"object1", "tabIndex", 6},
		{"ol1", "start", 1},
		{"td1", "colSpan", 2},
		{"td1", "rowSpan", 3},
		{"th1", "colSpan", 1},
		{"th1", "colSpan", 1},
		{"txtarea1", "rows", 10},
		{"txtarea1", "cols", 12},
		{"txtarea1", "maxLength", 128},
		{"txtarea1", "tabIndex", 4},
	}
	for _, test := range intTests {
		v, err := rt.RunString(`doc.find("#` + test.id + `").get(0).` + test.property + `()`)
		if err != nil {
			t.Errorf("Error for property name '%s' on element id '#%s':\n%+v", test.property, test.id, err)
		} else if v.Export() != int64(test.data) {
			t.Errorf("Expected %d for property name '%s' on element id '#%s'. Got %d", test.data, test.property, test.id, v.ToInteger())
		}
	}
}

func TestGenElementsNullProperties(t *testing.T) {
	t.Parallel()
	rt := getTestRuntimeWithDoc(t, testGenElems)
	nullTests := []struct {
		id       string
		property string
	}{
		{"audio2", "crossOrigin"},
		{"img3", "crossOrigin"},
		{"link2", "crossOrigin"},
	}

	for _, test := range nullTests {
		v, err := rt.RunString(`doc.find("#` + test.id + `").get(0).` + test.property + `()`)
		if err != nil {
			t.Errorf("Error for property name '%s' on element id '#%s':\n%+v", test.property, test.id, err)
		} else if v.Export() != nil {
			t.Errorf("Expected null for property name '%s' on element id '#%s'", test.property, test.id)
		}
	}
}

func TestGenElementsURLProperties(t *testing.T) {
	t.Parallel()
	rt, mi := getTestRuntimeAndModuleInstanceWithDoc(t, testGenElems)

	sel, parseError := mi.parseHTML(testGenElems)
	if parseError != nil {
		t.Errorf("Unable to parse html")
	}

	urlTests := []struct {
		id       string
		property string
		baseURL  string
		data     string
	}{
		{"a2", "href", "http://example.com/testpath", ""},
		{"a3", "href", "http://example.com", "http://example.com/relpath"},
		{"a3", "href", "http://example.com/somepath", "http://example.com/relpath"},
		{"a3", "href", "http://example.com/subdir/", "http://example.com/subdir/relpath"},
		{"a4", "href", "http://example.com/", "http://example.com/abspath"},
		{"a4", "href", "http://example.com/subdir/", "http://example.com/abspath"},
		{"a5", "href", "http://example.com/path?a=no-a&c=no-c", "http://example.com/path?a=yes-a&b=yes-b"},
		{"a6", "href", "http://example.com/path#oldfrag", "http://example.com/path#testfrag"},
		{"a7", "href", "http://example.com/prevdir/prevpath", "http://example.com/prtpath"},
		{"a8", "href", "http://example.com/testpath", "http://example.com/testpath"},
		{"base1", "href", "http://example.com", "http://example.com/foo.html"},
		{"base2", "href", "http://example.com", "http://example.com"},
		{"base3", "href", "http://example.com", "http://example.com"},
		{"audio1", "src", "http://example.com", "http://example.com/foo.wav"},
		{"audio2", "src", "http://example.com", ""},
		{"audio3", "src", "http://example.com", "http://example.com"},
		{"form1", "action", "http://example.com/", "http://example.com/submit_url"},
		{"form2", "action", "http://example.com/", ""},
		{"form3", "action", "http://example.com/", "http://example.com/"},
		{"iframe1", "src", "http://example.com", "http://example.com/testframe.html"},
		{"iframe2", "src", "http://example.com", ""},
		{"iframe3", "src", "http://example.com", "http://example.com"},
		{"img1", "src", "http://example.com", "http://example.com/test.png"},
		{"img2", "src", "http://example.com", ""},
		{"img3", "src", "http://example.com", "http://example.com"},
		{"input5", "src", "http://example.com", "http://example.com/input.png"},
		{"input5b", "src", "http://example.com", ""},
		{"input5c", "src", "http://example.com", "http://example.com"},
		{"link1", "href", "http://example.com", "http://example.com/test.css"},
		{"link2", "href", "http://example.com", ""},
		{"link3", "href", "http://example.com", "http://example.com"},
		{"object1", "data", "http://example.com", "http://example.com/test.png"},
		{"object2", "data", "http://example.com", ""},
		{"object3", "data", "http://example.com", "http://example.com"},
		{"script1", "src", "http://example.com", "http://example.com/script.js"},
		{"script2", "src", "http://example.com", ""},
		{"script3", "src", "http://example.com", "http://example.com"},
		{"src1", "src", "http://example.com", "http://example.com/test.png"},
		{"src2", "src", "http://example.com", ""},
		{"src3", "src", "http://example.com", "http://example.com"},
		{"track1", "src", "http://example.com", "http://example.com/foo.en.vtt"},
		{"track3", "src", "http://example.com", ""},
		{"track4", "src", "http://example.com", "http://example.com"},
	}
	for _, test := range urlTests {
		sel.URL = test.baseURL
		require.NoError(t, rt.Set("urldoc", sel))

		v, err := rt.RunString(`urldoc.find("#` + test.id + `").get(0).` + test.property + `()`)
		if err != nil {
			t.Errorf("Error for url property '%s' on element id '#%s':\n%+v", test.property, test.id, err)
		} else if v.Export() != test.data {
			t.Errorf("Expected '%s' for property name '%s' on element id '#%s', got '%s'", test.data, test.property, test.id, v.String())
		}
	}
}
