import { Kubernetes } from "k6/x/kubernetes";
import { describe, expect } from "https://jslib.k6.io/k6chaijs/4.3.4.3/index.js";
import { load, dump } from "https://cdn.jsdelivr.net/npm/js-yaml@4.1.0/dist/js-yaml.mjs";

let json = {
    apiVersion: "v1",
    kind: "Service",
    metadata: {
        name:      "json-service",
        namespace: "default",
    },
    spec: {
        selector: {
            app: "json-intg-test"
        },
        type: "ClusterIP",
        ports: [
            {
                name: "http",
                protocol: "TCP",
                port: 80,
                targetPort: 80,
            }
        ]
    }
}

let yaml = `
apiVersion: v1
kind: Service
metadata:
  name: yaml-service
  namespace: default
spec:
  selector:
    app: yaml-intg-test
  type: ClusterIP
  ports:
  - name: http
    protocol: TCP
    port: 80
    targetPort: 80
`

export default function () {
    const kubernetes = new Kubernetes();

    describe('JSON-based resources', () => {
        const name = json.metadata.name
        const ns = json.metadata.namespace

        let service

        describe('Create our Service using the JSON definition', () => {
            service = kubernetes.create(json)
            expect(service.metadata, 'new service').to.have.property('uid')
        })

        describe('Retrieve all available Services', () => {
            expect(kubernetes.list("Service", ns).length, 'total services').to.be.at.least(1)
        })

        describe('Retrieve our Service by name and namespace', () => {
            let fetched = kubernetes.get("Service", name, ns)
            expect(service.metadata.uid, 'created and fetched uids').to.equal(fetched.metadata.uid)
        })

        describe('Update our Service with a modified JSON definition', () => {
            const newValue = json.spec.selector.app + '-updated'
            json.spec.selector.app = newValue

            kubernetes.update(json)
            let updated = kubernetes.get("Service", name, ns)
            expect(updated.spec.selector.app, 'changed value').to.be.equal(newValue)
        })

        describe('Remove our Service to cleanup', () => {
            kubernetes.delete("Service", name, ns)
        })
    })

    describe('YAML-based resources', () => {
        let yamlObject = load(yaml)
        const name = yamlObject.metadata.name
        const ns = yamlObject.metadata.namespace

        describe('Create our Service using the YAML definition', () => {
            kubernetes.apply(yaml)
            let created = kubernetes.get("Service", name, ns)
            expect(created.metadata, 'new service').to.have.property('uid')
        })

        describe('Update our Service with a modified YAML definition', () => {
            const newValue = yamlObject.spec.selector.app + '-updated'
            yamlObject.spec.selector.app = newValue
            let newYaml = dump(yamlObject)

            kubernetes.apply(newYaml)
            let updated = kubernetes.get("Service", name, ns)
            expect(updated.spec.selector.app, 'changed value').to.be.equal(newValue)
        })

        describe('Remove our Service to cleanup', () => {
            kubernetes.delete("Service", name, ns)
        })
    })

}
