export default async function () {
  const jwk = {
    kty: "EC",
    crv: "P-521",
    x: "AVb0efjfHiCn_8BM5CDD4VSuJRmWvuQvA0uE1Bt0PzTkXzEbgTqc3sjNpZu7vTHUYLMpJSHnwbci5WZ8A9svrnU_",
    y: "AVAXNs_iRzlDINjkr8L9ObWpMxBhuB4iQSgrnheJGCK1t54FL0WXtZZD_Tk3nFG9USXE9IvD8CXOPNNpUyhsyzj7",
    d: "APQIdYNoupMPMPdq4FT-XNLOf9osn3am1DbPddZsRAv-YzHHwXKhJHgZPIJRSHvJEmP6UCF_hf9jb1nNVG46tIO0",
  };

  console.log("static key: " + JSON.stringify(jwk));

  const importedKey = await crypto.subtle.importKey(
    "jwk",
    jwk,
    { name: "ECDSA", namedCurve: "P-256" },
    true,
    ["sign"]
  );

  console.log("imported: " + JSON.stringify(importedKey));

  const exportedAgain = await crypto.subtle.exportKey("jwk", importedKey);

  console.log("exported again: " + JSON.stringify(exportedAgain));
}
