export default function () {
  const array = new Uint32Array(10);
  crypto.getRandomValues(array);

  for (const num of array) {
    console.log(num);
  }
}
