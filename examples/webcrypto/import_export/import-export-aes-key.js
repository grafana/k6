export default async function () {
  const generatedKey = await crypto.subtle.generateKey(
    {
      name: "AES-CBC",
      length: "256",
    },
    true,
    ["encrypt", "decrypt"]
  );

  const exportedKey = await crypto.subtle.exportKey("raw", generatedKey);

  const importedKey = await crypto.subtle.importKey(
    "raw",
    exportedKey,
    "AES-CBC",
    true,
    ["encrypt", "decrypt"]
  );

  console.log(JSON.stringify(importedKey));
}
