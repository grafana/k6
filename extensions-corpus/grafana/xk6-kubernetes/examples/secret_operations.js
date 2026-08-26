import { Kubernetes } from "k6/x/kubernetes";
import { describe, expect } from "https://jslib.k6.io/k6chaijs/4.3.4.3/index.js";
import { load, dump } from "https://cdn.jsdelivr.net/npm/js-yaml@4.1.0/dist/js-yaml.mjs";

let json = {
    apiVersion: "v1",
    kind: "Secret",
    metadata: {
        name:      "json-secret",
        namespace: "default",
    },
    type: "Opaque",
    data: {
        mysecret: "dGhlIHNlY3JldCB3b3JkIGlzLi4u",
    }
}

let yaml = `
apiVersion: v1
kind: Secret
metadata:
  name: yaml-secret
  namespace: default
type: Opaque
data:
  mysecret: dGhlIHNlY3JldCB3b3JkIGlzLi4u
`

export default function () {
    const kubernetes = new Kubernetes();

    describe('JSON-based resources', () => {
        const name = json.metadata.name
        const ns = json.metadata.namespace

        let secret

        describe('Create our Secret using the JSON definition', () => {
            secret = kubernetes.create(json)
            expect(secret.metadata, 'new secret').to.have.property('uid')
        })

        describe('Retrieve all available Secret', () => {
            expect(kubernetes.list("Secret", ns).length, 'total secrets').to.be.at.least(1)
        })

        describe('Retrieve our Secret by name and namespace', () => {
            let fetched = kubernetes.get("Secret", name, ns)
            expect(secret.metadata.uid, 'created and fetched uids').to.equal(fetched.metadata.uid)
        })

        describe('Update our Secret with a modified JSON definition', () => {
            const newValue = 'bmV3IHNlY3JldCB2YWx1ZQ=='
            json.data.mysecret = newValue

            kubernetes.update(json)
            let updated = kubernetes.get("Secret", name, ns)
            expect(updated.data.mysecret, 'changed value').to.be.equal(newValue)
        })

        describe('Remove our Secret to cleanup', () => {
            kubernetes.delete("Secret", name, ns)
        })
    })

    describe('YAML-based resources', () => {
        let yamlObject = load(yaml)
        const name = yamlObject.metadata.name
        const ns = yamlObject.metadata.namespace

        describe('Create our Secret using the YAML definition', () => {
            kubernetes.apply(yaml)
            let created = kubernetes.get("Secret", name, ns)
            expect(created.metadata, 'new secret').to.have.property('uid')
        })

        describe('Update our Secret with a modified YAML definition', () => {
            const newValue = 'bmV3IHNlY3JldCB2YWx1ZQ=='
            yamlObject.data.mysecret = newValue
            let newYaml = dump(yamlObject)

            kubernetes.apply(newYaml)
            let updated = kubernetes.get("Secret", name, ns)
            expect(updated.data.mysecret, 'changed value').to.be.equal(newValue)
        })

        describe('Remove our Secret to cleanup', () => {
            kubernetes.delete("Secret", name, ns)
        })
    })

}
