module github.com/loadimpact/k6

go 1.14

require (
	github.com/Azure/go-ntlmssp v0.0.0-20180810175552-4a21cbd618b4
	github.com/DataDog/datadog-go v0.0.0-20180330214955-e67964b4021a
	github.com/GeertJohan/go.rice v0.0.0-20170420135705-c02ca9a983da
	github.com/PuerkitoBio/goquery v1.3.0
	github.com/Shopify/sarama v1.19.0
	github.com/Shopify/toxiproxy v2.1.4+incompatible // indirect
	github.com/Soontao/goHttpDigestClient v0.0.0-20170320082612-6d28bb1415c5
	github.com/andybalholm/brotli v0.0.0-20190704151324-71eb68cc467c
	github.com/andybalholm/cascadia v1.0.0 // indirect
	github.com/daaku/go.zipexe v0.0.0-20150329023125-a5fe2436ffcb // indirect
	github.com/dlclark/regexp2 v1.4.1-0.20201116162257-a2a8dda75c91 // indirect
	github.com/dop251/goja v0.0.0-20201107160812-7545ac6de48a
	github.com/dustin/go-humanize v0.0.0-20171111073723-bb3d318650d4
	github.com/eapache/go-resiliency v1.1.0 // indirect
	github.com/eapache/queue v1.1.0 // indirect
	github.com/fatih/color v1.7.0
	github.com/gedex/inflector v0.0.0-20170307190818-16278e9db813 // indirect
	github.com/ghodss/yaml v1.0.0 // indirect
	github.com/gin-contrib/sse v0.0.0-20170109093832-22d885f9ecc7 // indirect
	github.com/gin-gonic/gin v1.1.5-0.20170702092826-d459835d2b07 // indirect
	github.com/go-sourcemap/sourcemap v2.1.3+incompatible // indirect
	github.com/golang/protobuf v1.4.2
	github.com/google/go-cmp v0.5.1 // indirect
	github.com/gorilla/websocket v1.4.2
	github.com/inconshreveable/mousetrap v1.0.0 // indirect
	github.com/influxdata/influxdb1-client v0.0.0-20191209144304-8bf82d3c094d
	github.com/jhump/protoreflect v1.7.0
	github.com/julienschmidt/httprouter v1.3.0
	github.com/kardianos/osext v0.0.0-20170510131534-ae77be60afb1 // indirect
	github.com/kelseyhightower/envconfig v1.4.0
	github.com/klauspost/compress v1.7.2
	github.com/klauspost/cpuid v1.3.1 // indirect
	github.com/kubernetes/helm v2.9.0+incompatible
	github.com/labstack/echo v3.2.6+incompatible // indirect
	github.com/labstack/gommon v0.2.2-0.20170925052817-57409ada9da0 // indirect
	github.com/mailru/easyjson v0.7.4-0.20200812114229-8ab5ff9cd8e4
	github.com/manyminds/api2go v0.0.0-20180125085803-95be7bd0455e
	github.com/mattn/go-colorable v0.0.9
	github.com/mattn/go-isatty v0.0.4
	github.com/mccutchen/go-httpbin v1.1.2-0.20190116014521-c5cb2f4802fa
	github.com/mitchellh/mapstructure v1.1.2
	github.com/nu7hatch/gouuid v0.0.0-20131221200532-179d4d0c4d8d
	github.com/onsi/ginkgo v1.14.0 // indirect
	github.com/oxtoacart/bpool v0.0.0-20150712133111-4e1c5567d7c2
	github.com/pierrec/xxHash v0.1.1 // indirect
	github.com/pkg/errors v0.9.1
	github.com/pmezard/go-difflib v1.0.0
	github.com/prometheus/client_model v0.2.0
	github.com/prometheus/common v0.15.0
	github.com/serenize/snaker v0.0.0-20171204205717-a683aaf2d516
	github.com/sirupsen/logrus v1.6.0
	github.com/spf13/afero v1.1.1
	github.com/spf13/cobra v0.0.4-0.20180629152535-a114f312e075
	github.com/spf13/pflag v1.0.1
	github.com/stretchr/testify v1.4.0
	github.com/tidwall/gjson v1.6.1
	github.com/tidwall/pretty v1.0.2
	github.com/ugorji/go v1.1.7 // indirect
	github.com/urfave/negroni v0.3.1-0.20180130044549-22c5532ea862
	github.com/valyala/bytebufferpool v1.0.0 // indirect
	github.com/valyala/fasttemplate v0.0.0-20170224212429-dcecefd839c4 // indirect
	github.com/zyedidia/highlight v0.0.0-20170330143449-201131ce5cf5
	golang.org/x/crypto v0.0.0-20200622213623-75b288015ac9
	golang.org/x/net v0.0.0-20201008223702-a5fa9d4b7c91
	golang.org/x/sys v0.0.0-20200930185726-fdedc70b468f // indirect
	golang.org/x/text v0.3.3
	golang.org/x/time v0.0.0-20191024005414-555d28b269f0
	golang.org/x/xerrors v0.0.0-20200804184101-5ec99f83aff1 // indirect
	google.golang.org/genproto v0.0.0-20200903010400-9bfcb5116336 // indirect
	google.golang.org/grpc v1.31.1
	google.golang.org/protobuf v1.24.0
	gopkg.in/go-playground/assert.v1 v1.2.1 // indirect
	gopkg.in/go-playground/validator.v8 v8.18.2 // indirect
	gopkg.in/guregu/null.v2 v2.1.2 // indirect
	gopkg.in/guregu/null.v3 v3.3.0
	gopkg.in/yaml.v2 v2.3.0
)

replace (
	github.com/davecgh/go-spew => github.com/davecgh/go-spew v1.1.0
	github.com/stretchr/testify => github.com/stretchr/testify v1.2.1
	github.com/ugorji/go => github.com/ugorji/go v0.0.0-20180112141927-9831f2c3ac10
	golang.org/x/text => golang.org/x/text v0.3.0
	gopkg.in/yaml.v2 => gopkg.in/yaml.v2 v2.1.1
)
