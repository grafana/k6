import http from 'k6/http';
import { sleep } from 'k6';

export default function () {
    var _ = undefined;

    while (true) {
        try {
            var result = eval(read_stdin("> "));
            _ = result;
            if (result !== undefined && result !== null) {
                console.log(result.toString())
            }
        } catch (error) {
            console.log(error.toString())
        }
    }
}
