import http from 'k6/http'

export const options = {
    tags: {
      customTag: 'MyValue',
    },
  };

export default function () {
   http.get("https://www.cloudflare.com/",{proto: "HTTP/3"})
}