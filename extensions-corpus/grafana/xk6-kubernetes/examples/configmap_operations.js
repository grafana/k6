import { Kubernetes } from "k6/x/kubernetes";
import { describe, expect } from "https://jslib.k6.io/k6chaijs/4.3.4.3/index.js";
import { load, dump } from "https://cdn.jsdelivr.net/npm/js-yaml@4.1.0/dist/js-yaml.mjs";

let json = {
    apiVersion: "v1",
    kind: "ConfigMap",
    metadata: {
        name:      "json-configmap",
        namespace: "default",
    },
    data: {
        K6_API_TEST_URL: "https://test.k6.io",
    }
}

let yaml = `
apiVersion: v1
kind: ConfigMap
metadata:
  name: yaml-configmap
  namespace: default
data:
  K6_API_TEST_URL: https://test.k6.io
`

export default function () {
    const kubernetes = new Kubernetes();

    describe('JSON-based resources', () => {
        const name = json.metadata.name
        const ns = json.metadata.namespace

        let configmap

        describe('Create our ConfigMap using the JSON definition', () => {
            configmap = kubernetes.create(json)
            expect(configmap.metadata, 'new configmap').to.have.property('uid')
        })

        describe('Retrieve all available ConfigMap', () => {
            expect(kubernetes.list("ConfigMap", ns).length, 'total configmaps').to.be.at.least(1)
        })

        describe('Retrieve our ConfigMap by name and namespace', () => {
            let fetched = kubernetes.get("ConfigMap", name, ns)
            expect(configmap.metadata.uid, 'created and fetched uids').to.equal(fetched.metadata.uid)
        })

        describe('Update our ConfigMap with a modified JSON definition', () => {
            const newValue = 'https://test-api.k6.io/'
            json.data.K6_API_TEST_URL = newValue

            kubernetes.update(json)
            let updated = kubernetes.get("ConfigMap", name, ns)
            expect(updated.data.K6_API_TEST_URL, 'changed value').to.be.equal(newValue)
        })

        describe('Remove our ConfigMap to cleanup', () => {
            kubernetes.delete("ConfigMap", name, ns)
        })
    })

    describe('YAML-based resources', () => {
        let yamlObject = load(yaml)
        const name = yamlObject.metadata.name
        const ns = yamlObject.metadata.namespace

        describe('Create our ConfigMap using the YAML definition', () => {
            kubernetes.apply(yaml)
            let created = kubernetes.get("ConfigMap", name, ns)
            expect(created.metadata, 'new configmap').to.have.property('uid')
        })

        describe('Update our ConfigMap with a modified YAML definition', () => {
            const newValue = 'https://test-api.k6.io/'
            yamlObject.data.K6_API_TEST_URL = newValue
            let newYaml = dump(yamlObject)

            kubernetes.apply(newYaml)
            let updated = kubernetes.get("ConfigMap", name, ns)
            expect(updated.data.K6_API_TEST_URL, 'changed value').to.be.equal(newValue)
        })

        describe('Remove our ConfigMap to cleanup', () => {
            kubernetes.delete("ConfigMap", name, ns)
        })
    })

}
