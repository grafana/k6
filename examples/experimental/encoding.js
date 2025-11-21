import { TextDecoder, TextEncoder } from "k6/experimental/encoding";

export default function () {
  const decoder = new TextDecoder("utf-8");
  const encoder = new TextEncoder();

  const decoded = decoder.decode(encoder.encode("hello world"));
  console.log(decoded);
}