import http from "k6/http";
import { sleep } from "k6";

let accessToken = "YOUR_GITHUB_ACCESS_TOKEN";

export default function() {

  let query = `
    query FindFirstIssue {
      repository(owner:"grafana", name:"k6") {
        issues(first:1) {
          edges {
            node {
              id
              number
              title
            }
          }
        }
      }
    }`;

  let headers = {
    'Authorization': `Bearer ${accessToken}`,
    "Content-Type": "application/json"
  };

  let res = http.post("https://api.github.com/graphql",
    JSON.stringify({ query: query }),
    {headers: headers}
  );

  if (res.status === 200) {
    console.log(JSON.stringify(res.body));
    let body = JSON.parse(res.body);
    let issue = body.data.repository.issues.edges[0].node;
    console.log(issue.id, issue.number, issue.title);

    let mutation = `
      mutation AddReactionToIssue {
        addReaction(input:{subjectId:"${issue.id}",content:HOORAY}) {
          reaction {
            content
          }
          subject {
            id
          }
        }
    }`;

    res = http.post("https://api.github.com/graphql",
      JSON.stringify({query: mutation}),
      {headers: headers}
    );
  }
  sleep(0.3);
}
