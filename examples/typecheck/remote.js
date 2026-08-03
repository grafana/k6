import Ajv from "https://esm.sh/ajv@6.12.5?bundle";

const ajv = new Ajv({ allErrors: true });
const validate = ajv.compile({
  type: "object",
  properties: {
    name: { type: "string" },
  },
  required: ["name"],
  additionalProperties: false,
});

export default function () {
  if (!validate({ name: "k6" })) {
    throw new Error(ajv.errorsText(validate.errors));
  }
}
