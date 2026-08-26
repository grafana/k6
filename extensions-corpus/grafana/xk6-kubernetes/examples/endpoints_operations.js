import { Kubernetes } from "k6/x/kubernetes";
import { describe, expect } from "https://jslib.k6.io/k6chaijs/4.3.4.3/index.js";
import { load } from "https://cdn.jsdelivr.net/npm/js-yaml@4.1.0/dist/js-yaml.mjs";

let json = {
    apiVersion: "v1",
    kind: "Endpoints",
    metadata: {
        name:      "json-endpoint",
        namespace: "default",
    },
    subsets: [
        {
            addresses: [
                {ip: "192.168.0.32"},
            ],
            ports: [
                {
                    name: "https",
                    port: 6443,
                    protocol: "TCP",
                }
            ],
        }
    ]
}

let yaml = `
apiVersion: v1
kind: Endpoints
metadata:
  name: yaml-endpoint
  namespace: default
subsets:
- addresses:
  - ip: 192.168.0.32
  ports:
  - name: https
    port: 6443
    protocol: TCP
`

export default function () {
    const kubernetes = new Kubernetes();

    describe('JSON-based resources', () => {
        const name = json.metadata.name
        const ns = json.metadata.namespace

        let endpoint

        describe('Create our Endpoints using the JSON definition', () => {
            endpoint = kubernetes.create(json)
            expect(endpoint.metadata, 'new endpoint').to.have.property('uid')
        })

        describe('Retrieve all available Endpoints', () => {
            expect(kubernetes.list("Endpoints", ns).length, 'total endpoints').to.be.at.least(1)
        })

        describe('Retrieve our Endpoints by name and namespace', () => {
            let fetched = kubernetes.get("Endpoints", name, ns)
            expect(endpoint.metadata.uid, 'created and fetched uids').to.equal(fetched.metadata.uid)
        })

        describe('Remove our Endpoints to cleanup', () => {
            kubernetes.delete("Endpoints", name, ns)
        })
    })

    describe('YAML-based resources', () => {
        let yamlObject = load(yaml)
        const name = yamlObject.metadata.name
        const ns = yamlObject.metadata.namespace

        describe('Create our Endpoints using the YAML definition', () => {
            kubernetes.apply(yaml)
            let created = kubernetes.get("Endpoints", name, ns)
            expect(created.metadata, 'new endpoint').to.have.property('uid')
        })

        describe('Remove our Endpoints to cleanup', () => {
            kubernetes.delete("Endpoints", name, ns)
        })
    })

}
