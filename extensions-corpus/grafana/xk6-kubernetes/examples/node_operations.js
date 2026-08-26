import { Kubernetes } from "k6/x/kubernetes";
import { describe, expect } from "https://jslib.k6.io/k6chaijs/4.3.4.3/index.js";

export default function () {
    const kubernetes = new Kubernetes();

    let nodes

    describe('Retrieve all available Nodes', () => {
        nodes = kubernetes.list("Node")
        expect(nodes.length, 'total nodes').to.be.at.least(1)
    })

    describe('Retrieve our Node by name', () => {
        let fetched = kubernetes.get("Node", nodes[0].metadata.name)
        expect(nodes[0].metadata.uid, 'fetched uids').to.equal(fetched.metadata.uid)
    })

}
