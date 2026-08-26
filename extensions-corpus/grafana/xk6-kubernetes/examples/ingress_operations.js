import { Kubernetes } from 'k6/x/kubernetes';
import { describe, expect } from 'https://jslib.k6.io/k6chaijs/4.3.4.3/index.js';
import { load, dump } from 'https://cdn.jsdelivr.net/npm/js-yaml@4.1.0/dist/js-yaml.mjs';

let json = {
    apiVersion: "networking.k8s.io/v1",
    kind: "Ingress",
    metadata: {
        name:      "json-ingress",
        namespace: "default",
    },
    spec: {
        ingressClassName: "nginx",
        rules: [
            {
                http: {
                    paths: [
                        {
                            path: "/my-service-path",
                            pathType: "Prefix",
                            backend: {
                                service: {
                                    name: "my-service",
                                    port: {
                                        number: 80
                                    }
                                }
                            }
                        }
                    ]
                }
            }
        ]
    }
}

let yaml = `
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: yaml-ingress
  namespace: default
spec:
  ingressClassName: nginx
  rules:
  - http:
      paths:
      - path: /my-service-path
        pathType: Prefix
        backend:
          service:
            name: my-service
            port:
              number: 80
`

export default function () {
    const kubernetes = new Kubernetes();

    describe('JSON-based resources', () => {
        const name = json.metadata.name
        const ns = json.metadata.namespace

        let ingress

        describe('Create our ingress using the JSON definition', () => {
            ingress = kubernetes.create(json)
            expect(ingress.metadata, 'new ingress').to.have.property('uid')
        })

        describe('Retrieve all available ingresses', () => {
            expect(kubernetes.list("Ingress.networking.k8s.io", ns).length, 'total ingresses').to.be.at.least(1)
        })

        describe('Retrieve our ingress by name and namespace', () => {
            let fetched = kubernetes.get("Ingress.networking.k8s.io", name, ns)
            expect(ingress.metadata.uid, 'created and fetched uids').to.equal(fetched.metadata.uid)
        })

        describe('Update our ingress with a modified JSON definition', () => {
            const newValue = json.spec.rules[0].http.paths[0].path + '-updated'
            json.spec.rules[0].http.paths[0].path = newValue

            kubernetes.update(json)
            let updated = kubernetes.get("Ingress.networking.k8s.io", name, ns)
            expect(updated.spec.rules[0].http.paths[0].path, 'changed value').to.be.equal(newValue)
            expect(updated.metadata.generation, 'ingress revision').to.be.at.least(2)
        })

        describe('Remove our ingresses to cleanup', () => {
            kubernetes.delete("Ingress.networking.k8s.io", name, ns)
        })
    })

    describe('YAML-based resources', () => {
        let yamlObject = load(yaml)
        const name = yamlObject.metadata.name
        const ns = yamlObject.metadata.namespace

        describe('Create our ingress using the YAML definition', () => {
            kubernetes.apply(yaml)
            let created = kubernetes.get("Ingress.networking.k8s.io", name, ns)
            expect(created.metadata, 'new ingress').to.have.property('uid')
        })

        describe('Update our ingress with a modified YAML definition', () => {
            const newValue = yamlObject.spec.rules[0].http.paths[0].path + '-updated'
            yamlObject.spec.rules[0].http.paths[0].path = newValue
            let newYaml = dump(yamlObject)

            kubernetes.apply(newYaml)
            let updated = kubernetes.get("Ingress.networking.k8s.io", name, ns)
            expect(updated.spec.rules[0].http.paths[0].path, 'changed value').to.be.equal(newValue)
            expect(updated.metadata.generation, 'ingress revision').to.be.at.least(2)
        })

        describe('Remove our ingresses to cleanup', () => {
            kubernetes.delete("Ingress.networking.k8s.io", name, ns)
        })
    })

}
