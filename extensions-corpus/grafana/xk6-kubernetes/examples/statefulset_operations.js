import { Kubernetes } from "k6/x/kubernetes";
import { describe, expect } from "https://jslib.k6.io/k6chaijs/4.3.4.3/index.js";
import { load, dump } from "https://cdn.jsdelivr.net/npm/js-yaml@4.1.0/dist/js-yaml.mjs";

let json = {
    apiVersion: "apps/v1",
    kind: "StatefulSet",
    metadata: {
        name:      "json-statefulset",
        namespace: "default",
    },
    spec: {
        replicas: 1,
        selector: {
            matchLabels: {
                app: "json-intg-test"
            }
        },
        template: {
            metadata: {
                labels: {
                    app: "json-intg-test"
                }
            },
            spec: {
                containers: [
                    {
                        name: "nginx",
                        image: "nginx:1.14.2",
                        ports: [
                            {containerPort: 80}
                        ]
                    }
                ]
            }
        }
    }
}

let yaml = `
apiVersion: apps/v1
kind: StatefulSet
metadata:
  name: yaml-statefulset
  namespace: default
spec:
  replicas: 1
  selector: 
    matchLabels:
      app: yaml-intg-test
  template:
    metadata:
      labels:
        app: yaml-intg-test
    spec:
      containers:
      - name: nginx
        image: nginx:1.14.2
        ports:
        - containerPort: 80
`

export default function () {
    const kubernetes = new Kubernetes();

    describe('JSON-based resources', () => {
        const name = json.metadata.name
        const ns = json.metadata.namespace

        let statefulset

        describe('Create our StatefulSet using the JSON definition', () => {
            statefulset = kubernetes.create(json)
            expect(statefulset.metadata, 'new statefulset').to.have.property('uid')
        })

        describe('Retrieve all available StatefulSets', () => {
            expect(kubernetes.list("StatefulSet.apps", ns).length, 'total statefulsets').to.be.at.least(1)
        })

        describe('Retrieve our StatefulSet by name and namespace', () => {
            let fetched = kubernetes.get("StatefulSet.apps", name, ns)
            expect(statefulset.metadata.uid, 'created and fetched uids').to.equal(fetched.metadata.uid)
        })

        describe('Update our StatefulSet with a modified JSON definition', () => {
            const newValue = 2
            json.spec.replicas = newValue

            kubernetes.update(json)
            let updated = kubernetes.get("StatefulSet.apps", name, ns)
            expect(updated.spec.replicas, 'changed value').to.be.equal(newValue)
        })

        describe('Remove our StatefulSet to cleanup', () => {
            kubernetes.delete("StatefulSet.apps", name, ns)
        })
    })

    describe('YAML-based resources', () => {
        let yamlObject = load(yaml)
        const name = yamlObject.metadata.name
        const ns = yamlObject.metadata.namespace

        describe('Create our StatefulSet using the YAML definition', () => {
            kubernetes.apply(yaml)
            let created = kubernetes.get("StatefulSet.apps", name, ns)
            expect(created.metadata, 'new statefulset').to.have.property('uid')
        })

        describe('Update our StatefulSet with a modified YAML definition', () => {
            const newValue = 2
            yamlObject.spec.replicas = newValue
            let newYaml = dump(yamlObject)

            kubernetes.apply(newYaml)
            let updated = kubernetes.get("StatefulSet.apps", name, ns)
            expect(updated.spec.replicas, 'changed value').to.be.equal(newValue)
        })

        describe('Remove our StatefulSet to cleanup', () => {
            kubernetes.delete("StatefulSet.apps", name, ns)
        })
    })

}
