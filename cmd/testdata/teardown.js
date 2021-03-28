import { abortTest } from 'k6';

export default function () {
    abortTest();
}

export function teardown() {
    console.log('Calling teardown function after abortTest()');
}