import http from 'k6/http'

export const options = {
    tags: {
      customTag: 'MyValue',
    },
  };

export default function () {
    let resp;
    resp = http.get('https://www.cloudflare.com/')
    console.log(resp.proto) // Prints HTTP/2.0

    resp = http.get('https://www.cloudflare.com/',{proto: "HTTP/3"})
    console.log(resp.proto) // Prints HTTP/3.0
}