import http from "k6/http";
import { browser } from "k6/browser";
import { sleep } from "k6";

export const options = {
    scenarios: {
        ui: {
            executor: "shared-iterations",
            options: {
                browser: {
                    type: "chromium",
                },
            },
        },
    },
};

export default async function () {
    var _ = undefined;
    var g = global;

    while (true) {
        try {
            var input = read_stdin("> ");

            // Special syntax: '%set foo = 123' actually evaluates
            // 'g.foo = 123', which sets the value in globals.
            // The value can then be referenced by just using 'foo',
            // as if we had run 'var foo = 123'.
            if (input.startsWith("%set ")) {
                input = "g." + input.substring(4).trim();
            }

            // Need to do this in order to allow using 'await'.
            var code = `(async function() { return ${input} })`;

            var result = await eval(code)();
            _ = result;
            if (result !== undefined && result !== null) {
                console.log(result.toString());
            }
        } catch (error) {
            console.error(error.toString());
            input = input.trim();
            if (input.startsWith("let") || input.startsWith("const") || input.startsWith("var")) {
                console.info("Hint: the REPL only supports expressions (e.g. `1 + 1`), not statements (e.g. `var foo = 123`).");
                console.info("Hint: In order to set a variable globally, use `%foo = 123`.");
            }
        }
    }
}
