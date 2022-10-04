# Intro

A connected graph of string to string pairs (map[string]string) with amortized construction costs and O(1) equality cost.

The O(1) equality comes from comparing the pointers to the struct instead of the struct itself.

Amortized cost mean that as time pass on and you stop adding more pairs the fairly high initial costs go down.

Additionally this should considerably reduce GC [citation needed] as we don't continuously make new objects.


# TODO 
- Count the number of nodes, for example in the root.
- Do not have any of this in the Go's memory as this just adds to things the GC needs to check but will in practice never free as we keep this memory around forever.
- Try to have a GC for nodes that haven't been accessed in a long while.
