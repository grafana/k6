import { open } from "k6/experimental/fs";
import { sleep } from "k6";

export const options = {
  vus: 10,
  iterations: 10,
};

let file;
(async function () {
    file = await open("bonjour.txt");
})();

export default async function () {
    file.close();
    sleep(10);
}