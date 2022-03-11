# circonusllhist

A golang implementation of the OpenHistogram [libcircllhist](https://github.com/openhistogram/libcircllhist) library.

[![godocs.io](http://godocs.io/github.com/openhistogram/circonusllhist?status.svg)](http://godocs.io/github.com/openhistogram/circonusllhist)
<!-- [![Go Reference](https://pkg.go.dev/badge/github.com/openhistogram/circonusllhist.svg)](https://pkg.go.dev/github.com/openhistogram/circonusllhist) -->

## Overview

Package `circllhist` provides an implementation of OpenHistogram's fixed log-linear histogram data structure.  This allows tracking of histograms in a composable way such that accurate error can be reasoned about.

## License

[Apache 2.0](LICENSE)

<!--
## Documentation

More complete docs can be found at [pkg.go.dev](https://pkg.go.dev/github.com/openhistogram/circonusllhist)
-->

## Usage Example

```go
package main

import (
	"fmt"

	"github.com/openhistogram/circonusllhist"
)

func main() {
	//Create a new histogram
	h := circonusllhist.New()

	//Insert value 123, three times
	if err := h.RecordValues(123, 3); err != nil {
		panic(err)
	}

	//Insert 1x10^1
	if err := h.RecordIntScale(1, 1); err != nil {
		panic(err)
	}

	//Print the count of samples stored in the histogram
	fmt.Printf("%d\n", h.Count())

	//Print the sum of all samples
	fmt.Printf("%f\n", h.ApproxSum())
}
```

### Usage Without Lookup Tables

By default, bi-level sparse lookup tables are used in this OpenHistogram implementation to improve insertion time by about 20%. However, the size of these tables ranges from a minimum of ~0.5KiB to a maximum of ~130KiB. While usage nearing the theoretical maximum is unlikely, as the lookup tables are kept as sparse tables, normal usage will be above the minimum. For applications where insertion time is not the most important factor and memory efficiency is, especially when datasets contain large numbers of individual histograms, opting out of the lookup tables is an appropriate choice. Generate new histograms without lookup tables like:

```go
package main

import "github.com/openhistogram/circonusllhist"

func main() {
	//Create a new histogram without lookup tables
	h := circonusllhist.New(circonusllhist.NoLookup())
	// ...
}
```

#### Notes on Serialization

When intentionally working without lookup tables, care must be taken to correctly serialize and deserialize the histogram data. The following example creates a histogram without lookup tables, serializes and deserializes it manually while never allocating any excess memory:

```go
package main

import (
	"bytes"
	"fmt"
	
	"github.com/openhistogram/circonusllhist"
)

func main() {
	// create a new histogram without lookup tables
	h := circonusllhist.New(circonusllhist.NoLookup())
	if err := h.RecordValue(1.2); err != nil {
		panic(err)
	}

	// serialize the histogram 
	var buf bytes.Buffer
	if err := h.Serialize(&buf); err != nil {
		panic(err)
    }
	
    // deserialize into a new histogram
	h2, err := circonusllhist.DeserializeWithOptions(&buf, circonusllhist.NoLookup())
	if err != nil {
		panic(err)
	}
	
	// the two histograms are equal
	fmt.Println(h.Equals(h2))
}
```

While the example above works cleanly when manual (de)serialization is required, a different approach is needed when implicitly (de)serializing histograms into a JSON format. The following example creates a histogram without lookup tables, serializes and deserializes it implicitly using Go's JSON library, ensuring no excess memory allocations occur:

```go
package main

import (
	"encoding/json"
	"fmt"
	
	"github.com/openhistogram/circonusllhist"
)

func main() {
	// create a new histogram without lookup tables
	h := circonusllhist.New(circonusllhist.NoLookup())
	if err := h.RecordValue(1.2); err != nil {
		panic(err)
	}

	// serialize the histogram
	data, err := json.Marshal(h)
	if err != nil {
		panic(err)
    }
	
    // deserialize into a new histogram
    var wrapper2 circonusllhist.HistogramWithoutLookups
	if err := json.Unmarshal(data, &wrapper2); err != nil {
		panic(err)
	}
	h2 := wrapper2.Histogram()
	
	// the two histograms are equal
	fmt.Println(h.Equals(h2))
}
```

Once the `circonusllhist.HistogramWithoutLookups` wrapper has been used as a deserialization target, the underlying histogram may be extracted with the `Histogram()` method. It is also possible to extract the histogram while allocating memory for lookup tables if necessary with the `HistogramWithLookups()` method.
