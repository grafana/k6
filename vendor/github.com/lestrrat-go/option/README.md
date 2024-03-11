# option

Base object for the "Optional Parameters Pattern".

# DESCRIPTION

The beauty of this pattern is that you can achieve a method that can
take the following simple calling style

```go
obj.Method(mandatory1, mandatory2)
```

or the following, if you want to modify its behavior with optional parameters

```go
obj.Method(mandatory1, mandatory2, optional1, optional2, optional3)
```

Instead of the more clunky zero value for optionals style

```go
obj.Method(mandatory1, mandatory2, nil, "", 0)
```

or the equally clunky config object style, which requires you to create a
struct with `NamesThatLookReallyLongBecauseItNeedsToIncludeMethodNamesConfig	

```go
cfg := &ConfigForMethod{
 Optional1: ...,
 Optional2: ...,
 Optional3: ...,
}
obj.Method(mandatory1, mandatory2, &cfg)
```

# SYNOPSIS 

Create an "identifier" for the option. We recommend using an unexported empty struct,
because

1. It is uniquely identifiable globally
1. Takes minimal space
1. Since it's unexported, you do not have to worry about it leaking elsewhere or having it changed by consumers

```go
// an unexported empty struct
type identFeatureX struct{} 
```

Then define a method to create an option using this identifier. Here we assume
that the option will be a boolean option.

```go
// this is optional, but for readability we usually use a wrapper
// around option.Interface, or a type alias.
type Option
func WithFeatureX(v bool) Option {
  // use the constructor to create a new option
  return option.New(identFeatureX{}, v)
}
```

Now you can create an option, which essentially a two element tuple consisting
of an identifier and its associated value.

To consume this, you will need to create a function with variadic parameters,
and iterate over the list looking for a particular identifier:

```go
func MyAwesomeFunc( /* mandatory parameters omitted */, options ...[]Option) {
  var enableFeatureX bool
  // The nolint directive is recommended if you are using linters such
  // as golangci-lint
  //nolint:forcetypeassert 
  for _, option := range options {
    switch option.Ident() {
    case identFeatureX{}:
      enableFeatureX = option.Value().(bool)
    // other cases omitted
    }
  }
  if enableFeatureX {
    ....
  }
}
```

# Option objects

Option objects take two arguments, its identifier and the value it contains.

The identifier can be anything, but it's usually better to use a an unexported
empty struct so that only you have the ability to generate said option:

```go
type identOptionalParamOne struct{}
type identOptionalParamTwo struct{}
type identOptionalParamThree struct{}

func WithOptionOne(v ...) Option {
	return option.New(identOptionalParamOne{}, v)
}
```

Then you can call the method we described above as

```go
obj.Method(m1, m2, WithOptionOne(...), WithOptionTwo(...), WithOptionThree(...))
```

Options should be parsed in a code that looks somewhat like this

```go
func (obj *Object) Method(m1 Type1, m2 Type2, options ...Option) {
  paramOne := defaultValueParamOne
  for _, option := range options {
    switch option.Ident() {
    case identOptionalParamOne{}:
      paramOne = option.Value().(...)
    }
  }
  ...
}
```

The loop requires a bit of boilerplate, and admittedly, this is the main downside
of this module. However, if you think you want use the Option as a Function pattern,
please check the FAQ below for rationale.

# Simple usage

Most of the times all you need to do is to declare the Option type as an alias
in your code:

```go
package myawesomepkg

import "github.com/lestrrat-go/option"

type Option = option.Interface
```

Then you can start defining options like they are described in the SYNOPSIS section.

# Differentiating Options

When you have multiple methods and options, and those options can only be passed to
each one the methods, it's hard to see which options should be passed to which method.

```go
func WithX() Option { ... }
func WithY() Option { ... }

// Now, which of WithX/WithY go to which method?
func (*Obj) Method1(options ...Option) {}
func (*Obj) Method2(options ...Option) {}
```

In this case the easiest way to make it obvious is to put an extra layer around
the options so that they have different types

```go
type Method1Option interface {
  Option
  method1Option()
}

type method1Option struct { Option }
func (*method1Option) method1Option() {}

func WithX() Method1Option {
  return &methodOption{option.New(...)}
}

func (*Obj) Method1(options ...Method1Option) {}
```

This way the compiler knows if an option can be passed to a given method.

# FAQ

## Why aren't these function-based?

Using a base option type like `type Option func(ctx interface{})` is certainly one way to achieve the same goal. In this case, you are giving the option itself the ability to "configure" the main object. For example:

```go
type Foo struct {
  optionaValue bool
}

type Option func(*Foo) error

func WithOptionalValue(v bool) Option {
  return Option(func(f *Foo) error {
    f.optionalValue = v
    return nil
  })
}

func NewFoo(options ...Option) (*Foo, error) {
  var f Foo
  for _, o := range options {
    if err := o(&f); err != nil {
      return nil, err
    }
  }
  return &f
}
```

This in itself is fine, but we think there are a few problems:

### 1. It's hard to create a reusable "Option" type

We create many libraries using this optional pattern. We would like to provide a default base object. However, this function based approach is not reusuable because each "Option" type requires that it has a context-specific input type. For example, if the "Option" type in the previous example was `func(interface{}) error`, then its usability will significantly decrease because of the type conversion.

This is not to say that this library's approach is better as it also requires type conversion to convert the _value_ of the option. However, part of the beauty of the original function based approach was the ease of its use, and we claim that this significantly decreases the merits of the function based approach.

### 2. The receiver requires exported fields

Part of the appeal for a function-based option pattern is by giving the option itself the ability to do what it wants, you open up the possibility of allowing third-parties to create options that do things that the library authors did not think about.

```go
package thirdparty
, but when I read drum sheet music, I kind of get thrown off b/c many times it says to hit the bass drum where I feel like it's a snare hit.
func WithMyAwesomeOption( ... ) mypkg.Option  {
  return mypkg.Option(func(f *mypkg) error {
    f.X = ...
    f.Y = ...
    f.Z = ...
    return nil
  })
}
```

However, for any third party code to access and set field values, these fields (`X`, `Y`, `Z`) must be exported. Basically you will need an "open" struct.

Exported fields are absolutely no problem when you have a struct that represents data alone (i.e., API calls that refer or change state information) happen, but we think that casually expose fields for a library struct is a sure way to maintenance hell in the future. What happens when you want to change the API? What happens when you realize that you want to use the field as state (i.e. use it for more than configuration)? What if they kept referring to that field, and then you have concurrent code accessing it?

Giving third parties complete access to exported fields is like handing out a loaded weapon to the users, and you are at their mercy.

Of course, providing public APIs for everything so you can validate and control concurrency is an option, but then ... it's a lot of work, and you may have to provide APIs _only_ so that users can refer it in the option-configuration phase. That sounds like a lot of extra work.

