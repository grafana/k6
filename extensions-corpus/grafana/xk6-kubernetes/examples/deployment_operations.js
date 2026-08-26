import { Kubernetes } from "k6/x/kubernetes";
import { describe, expect } from "https://jslib.k6.io/k6chaijs/4.3.4.3/index.js";
import { load, dump } from "https://cdn.jsdelivr.net/npm/js-yaml@4.1.0/dist/js-yaml.mjs";

let json = {
    apiVersion: "apps/v1",
    kind: "Deployment",
    metadata: {
        name:      "json-deployment",
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
kind: Deployment
metadata:
  name: yaml-deployment
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

        let deployment

        describe('Create our Deployment using the JSON definition', () => {
            deployment = kubernetes.create(json)
            expect(deployment.metadata, 'new deployment').to.have.property('uid')
        })

        describe('Retrieve all available Deployments', () => {
            expect(kubernetes.list("Deployment.apps", ns).length, 'total deployments').to.be.at.least(1)
        })

        describe('Retrieve our Deployment by name and namespace', () => {
            let fetched = kubernetes.get("Deployment.apps", name, ns)
            expect(deployment.metadata.uid, 'created and fetched uids').to.equal(fetched.metadata.uid)
        })

        describe('Update our Deployment with a modified JSON definition', () => {
            const newValue = 2
            json.spec.replicas = newValue

            kubernetes.update(json)
            let updated = kubernetes.get("Deployment.apps", name, ns)
            expect(updated.spec.replicas, 'changed value').to.be.equal(newValue)
        })

        describe('Remove our Deployment to cleanup', () => {
            kubernetes.delete("Deployment.apps", name, ns)
        })
    })

    describe('YAML-based resources', () => {
        let yamlObject = load(yaml)
        const name = yamlObject.metadata.name
        const ns = yamlObject.metadata.namespace

        describe('Create our Deployment using the YAML definition', () => {
            kubernetes.apply(yaml)
            let created = kubernetes.get("Deployment.apps", name, ns)
            expect(created.metadata, 'new deployment').to.have.property('uid')
        })

        describe('Update our Deployment with a modified YAML definition', () => {
            const newValue = 2
            yamlObject.spec.replicas = newValue
            let newYaml = dump(yamlObject)

            kubernetes.apply(newYaml)
            let updated = kubernetes.get("Deployment.apps", name, ns)
            expect(updated.spec.replicas, 'changed value').to.be.equal(newValue)
        })

        describe('Remove our Deployment to cleanup', () => {
            kubernetes.delete("Deployment.apps", name, ns)
        })
    })

}
