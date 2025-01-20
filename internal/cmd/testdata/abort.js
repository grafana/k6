import exec from 'k6/execution';

export default function () {
    exec.test.abort();
}
