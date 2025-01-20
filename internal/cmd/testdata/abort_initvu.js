import exec from 'k6/execution';

// This won't fail on initial parsing of the script, but on VU initialization.
if (__VU == 1) {
  exec.test.abort();
}

export default function() {}
