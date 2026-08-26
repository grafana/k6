# Design Documents

Design documents are intended mainly for new features and major refactoring efforts. The goal is, besides analyzing the problem scope and reaching a consensus, to ensure transparency and collect user feedback early. 

Design proposals are stored under the [`docs/01-development/design-docs`](https://github.com/grafana/xk6-disruptor/tree/master/docs/01-development/design-docs) folder in this repository. The structure of the design document is described in [000-TEMPLATE.md](./000-TEMPLATE.md).

## How to initiate a new design document

Design documents follows the process described below.

1. Create a new branch and use [`000-TEMPLATE.md`](./000-TEMPLATE.md) as a template for the new document.
2. The naming convention for the design document is `nnn-topic.md`, where `nnn` is the next number in sequence.
3. Make a PR and label it with `design-proposal`. If you want to collaborate in the proposal before sharing it (e.g. multiple authors) we strongly recommend creating a Draft PR and be sure the document's status is also set to Draft. This will prevent comments on unfinished proposals.
4. Once you're ready to share the document, move the status of the proposal to "In Discussion" and mark the PR as ready.
5. Add the approvers as reviewers of the PR
6. If the proposal introduces a new feature or breaking changes to the API, it is advisable to announce the PR in the relevant channels to reach a wider audience and gather more feedback (e.g. [k6 community forum](https://community.k6.io/) or [k6 slack workspace](https://k6io.slack.com/))
