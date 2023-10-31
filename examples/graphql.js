// Import necessary modules from k6
import http from "k6/http";
import { sleep } from "k6";

// Replace with your GitHub Personal Access Token
let accessToken = "YOUR_GITHUB_ACCESS_TOKEN";

// Define the default k6 function
export default function() {

  // GraphQL query to find the first issue in the "grafana/k6" repository
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

  // Set request headers, including the authorization token
  let headers = {
    'Authorization': `Bearer ${accessToken}`,
    "Content-Type": "application/json"
  };

  // Send a POST request to the GitHub GraphQL API
  let res = http.post("https://api.github.com/graphql",
    JSON.stringify({ query: query }),
    {headers: headers}
  );

  // Check if the response status is 200 (OK)
  if (res.status === 200) {
    // Log the response body
    console.log(JSON.stringify(res.body));
    
    // Parse the JSON response
    let body = JSON.parse(res.body);
    
    // Extract the first issue information
    let issue = body.data.repository.issues.edges[0].node;
    
    // Log the issue details
    console.log(issue.id, issue.number, issue.title);

    // GraphQL mutation to add a reaction ("HOORAY") to the issue
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

    // Send a POST request to the GitHub GraphQL API to add a reaction
    res = http.post("https://api.github.com/graphql",
      JSON.stringify({query: mutation}),
      {headers: headers}
    );
  }

  // Sleep for 0.3 seconds to simulate user pacing
  sleep(0.3);
}
