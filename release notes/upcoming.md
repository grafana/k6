TODO: Intro

## New Features!
* New JS API to set seed for PRNG. Now, you are able to set a seed to get reproducible (pseudo-)random numbers. (#677)

```js
import {randomSeed} from "k6";

randomSeed(123456789);
let rnd = Math.random();
console.log(rnd)
```

### Category: Title (#533)

Description of feature.

**Docs**: [Title](http://k6.readme.io/docs/TODO)

## UX
* New option to reset cloud token (#672)

## Bugs fixed!

* Category: description of bug. (#PR)