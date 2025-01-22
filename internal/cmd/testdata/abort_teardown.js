import exec from 'k6/execution';

export default function () {
    exec.test.abort();
}

export function teardown() {
    console.log('Calling teardown function after test.abort()');
}
