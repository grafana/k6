import secrets from "k6/secrets";

export default async () => {
  const my_secret = await secrets.get("cool");
  console.log(my_secret == "cool secret");
  const anothersource = await secrets.source("another")
  console.log(await anothersource.get("cool") == "cool secret");
  console.log(await anothersource.get("cool") == "not cool secret");
}
