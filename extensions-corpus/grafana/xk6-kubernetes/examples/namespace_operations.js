import { Kubernetes } from 'k6/x/kubernetes';
import { describe, expect } from 'https://jslib.k6.io/k6chaijs/4.3.4.3/index.js';
import { load, dump } from 'https://cdn.jsdelivr.net/npm/js-yaml@4.1.0/dist/js-yaml.mjs';

let json = {
    apiVersion: "v1",
    kind: "Namespace",
    metadata: {
        name: "json-namespace",
        labels: {
            "k6.io/created_by": "xk6-kubernetes",
        }
    }
}

let yaml = `
apiVersion: v1
kind: Namespace
metadata:
  name: yaml-namespace
  labels:
    k6.io/created_by: xk6-kubernetes
`

export default function () {
    const kubernetes = new Kubernetes();

    describe('JSON-based resources', () => {
        const name = json.metadata.name

        let namespace

        describe('Create our Namespace using the JSON definition', () => {
            namespace = kubernetes.create(json)
            expect(namespace.metadata, 'new namespace').to.have.property('uid')
        })

        describe('Retrieve all available Namespaces', () => {
            expect(kubernetes.list("Namespace").length, 'total namespaces').to.be.at.least(1)
        })

        describe('Retrieve our Namespace by name', () => {
            let fetched = kubernetes.get("Namespace", name)
            expect(namespace.metadata.uid, 'created and fetched uids').to.equal(fetched.metadata.uid)
        })

        describe('Update our Namespace with a modified JSON definition', () => {
            const newValue = "xk6-kubernetes-example"
            json.metadata.labels["k6.io/created_by"] = newValue

            kubernetes.update(json)
            let updated = kubernetes.get("Namespace", name)
            expect(updated.metadata.labels["k6.io/created_by"], 'changed value').to.be.equal(newValue)
        })

        describe('Remove our Namespace to cleanup', () => {
            kubernetes.delete("Namespace", name)
        })
    })

    describe('YAML-based resources', () => {
        let yamlObject = load(yaml)
        const name = yamlObject.metadata.name

        describe('Create our Namespace using the YAML definition', () => {
            kubernetes.apply(yaml)
            let created = kubernetes.get("Namespace", name)
            expect(created.metadata, 'new namespace').to.have.property('uid')
        })

        describe('Update our Namespace with a modified YAML definition', () => {
            const newValue = "xk6-kubernetes-example"
            yamlObject.metadata.labels["k6.io/created_by"] = newValue
            let newYaml = dump(yamlObject)

            kubernetes.apply(newYaml)
            let updated = kubernetes.get("Namespace", name)
            expect(updated.metadata.labels["k6.io/created_by"], 'changed value').to.be.equal(newValue)
        })

        describe('Remove our Namespace to cleanup', () => {
            kubernetes.delete("Namespace", name)
        })
    })

}
