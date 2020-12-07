# go.rice

[![Build Status](https://travis-ci.org/GeertJohan/go.rice.png)](https://travis-ci.org/GeertJohan/go.rice)
[![Godoc](https://img.shields.io/badge/godoc-go.rice-blue.svg?style=flat-square)](https://godoc.org/github.com/GeertJohan/go.rice)

go.rice is a [Go](http://golang.org) package that makes working with resources such as html,js,css,images and templates easy. During development `go.rice` will load required files directly from disk. Upon deployment it's easy to add all resource files to a executable using the `rice` tool, without changing the source code for your package. go.rice provides methods to add resources to a binary in different scenarios.

## What does it do

The first thing go.rice does is finding the correct absolute path for your resource files. Say you are executing a binary in your home directory, but your `html-files` are in `$GOPATH/src/yourApplication/html-files`. `go.rice` will lookup the correct path for that directory (relative to the location of yourApplication). All you have to do is include the resources using `rice.FindBox("html-files")`.

This works fine when the source is available to the machine executing the binary, which is the case when installing the executable with `go get` or `go install`. But it does not work when you wish to provide a single binary without source. This is where the `rice` tool comes in. It analyses source code and finds call's to `rice.FindBox(..)`. Then it adds the required directories to the executable binary, There are two strategies to do this. You can 'embed' the assets by generating go source code and then compile them into the executable binary, or you can 'append' the assets to the executable binary after compiling. In both cases the `rice.FindBox(..)` call detects the embedded or appended resources and load those, instead of looking up files from disk.

## Installation

Use `go get` to install the package the `rice` tool.

```bash
go get github.com/GeertJohan/go.rice
go get github.com/GeertJohan/go.rice/rice
```

## Package usage

Import the package: `import "github.com/GeertJohan/go.rice"`

Serving a static content folder over HTTP with a rice Box:

```go
http.Handle("/", http.FileServer(rice.MustFindBox("http-files").HTTPBox()))
http.ListenAndServe(":8080", nil)
```

Serve a static content folder over HTTP at a non-root location:

```go
box := rice.MustFindBox("cssfiles")
cssFileServer := http.StripPrefix("/css/", http.FileServer(box.HTTPBox()))
http.Handle("/css/", cssFileServer)
http.ListenAndServe(":8080", nil)
```

Note the *trailing slash* in `/css/` in both the call to
`http.StripPrefix` and `http.Handle`.

Loading a template:

```go
// find a rice.Box
templateBox, err := rice.FindBox("example-templates")
if err != nil {
	log.Fatal(err)
}
// get file contents as string
templateString, err := templateBox.String("message.tmpl")
if err != nil {
	log.Fatal(err)
}
// parse and execute the template
tmplMessage, err := template.New("message").Parse(templateString)
if err != nil {
	log.Fatal(err)
}
tmplMessage.Execute(os.Stdout, map[string]string{"Message": "Hello, world!"})

```

Never call `FindBox()` or `MustFindBox()` from an `init()` function, as there is no guarantee the boxes are loaded at that time.

### Calling FindBox and MustFindBox

Always call `FindBox()` or `MustFindBox()` with string literals e.g. `FindBox("example")`. Do not use string constants or variables. This will prevent the rice tool to fail with error `Error: found call to rice.FindBox, but argument must be a string literal.`.

## Tool usage

The `rice` tool lets you add the resources to a binary executable so the files are not loaded from the filesystem anymore. This creates a 'standalone' executable. There are multiple strategies to add the resources and assets to a binary, each has pro's and con's but all will work without requiring changes to the way you load the resources.

### `rice embed-go`: Embed resources by generating Go source code

Execute this method before building. It generates a single Go source file called *rice-box.go* for each package. The generated go file contains all assets. The Go tool compiles this into the binary.

The downside with this option is that the generated go source file can become large, which may slow down compilation and requires more memory to compile.

Execute the following commands:

```bash
rice embed-go
go build
```

*A Note on Symbolic Links*: `embed-go` uses the `os.Walk` function from the standard library.  The `os.Walk` function does **not** follow symbolic links. When creating a box, be aware that any symbolic links inside your box's directory are not followed. When the box itself is a symbolic link, the rice tool resolves its actual location before adding the contents.

### `rice embed-syso`: Embed resources by generating a coff .syso file and some .go source code

** This method is experimental. Do not use for production systems. **

Execute this method before building. It generates a COFF .syso file and Go source file. The Go compiler then compiles these files into the binary.

Execute the following commands:

```bash
rice embed-syso
go build
```

### `rice append`: Append resources to executable as zip file

This method changes an already built executable. It appends the resources as zip file to the binary. It makes compilation a lot faster. Using the append method works great for adding large assets to an executable binary.

A downside for appending is that it does not provide a working Seek method.

Run the following commands to create a standalone executable.

```bash
go build -o example
rice append --exec example
```

## Help information

Run `rice --help` for information about all flags and subcommands.

You can use the `--help` flag on each sub-command. For example: `rice append --help`.

## Order of precedence

When opening a new box, the `rice.FindBox(..)` tries to locate the resources in the following order:

- embedded (generated as `rice-box.go`)
- appended (appended to the binary executable after compiling)
- 'live' from filesystem

## License

This project is licensed under a Simplified BSD license. Please read the [LICENSE file][license].

## Package documentation

You will find package documentation at [godoc.org/github.com/GeertJohan/go.rice][godoc].

[license]: https://github.com/GeertJohan/go.rice/blob/master/LICENSE
[godoc]: http://godoc.org/github.com/GeertJohan/go.rice
