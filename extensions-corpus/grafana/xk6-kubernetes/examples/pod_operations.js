import { Kubernetes } from 'k6/x/kubernetes';
import { describe, expect } from 'https://jslib.k6.io/k6chaijs/4.3.4.3/index.js';
import { load, dump } from 'https://cdn.jsdelivr.net/npm/js-yaml@4.1.0/dist/js-yaml.mjs';

let json = {
    apiVersion: "v1",
    kind: "Pod",
    metadata: {
        name:      "json-pod",
        namespace: "default"
    },
    spec: {
        containers: [
            {
                name: "busybox",
                image: "busybox",
                command: ["sh", "-c", "sleep 30"]
            }
        ]
    }
}

let yaml = `
apiVersion: v1
kind: Pod
metadata:
  name: yaml-pod
  namespace: default
spec:
  containers:
  - name: busybox
    image: busybox
    command: ["sh",  "-c", "sleep 30"]
`

export default function () {
    const kubernetes = new Kubernetes();

    describe('JSON-based resources', () => {
        const name = json.metadata.name
        const ns = json.metadata.namespace
        const helpers = kubernetes.helpers(ns)

        let pod

        describe('Create our pod using the JSON definition and wait until running', () => {
            pod = kubernetes.create(json)
            expect(pod.metadata, 'new pod').to.have.property('uid')
            expect(pod.status.phase, 'new pod status').to.equal('Pending')

            helpers.waitPodRunning(name, 10)

            let fetched = kubernetes.get("Pod", name, ns)
            expect(fetched.status.phase, 'status after waiting').to.equal('Running')
        })

        describe('Retrieve all available pods', () => {
            expect(kubernetes.list("Pod", ns).length, 'total pods').to.be.at.least(1)
        })

        describe('Retrieve our pod by name and namespace, then execute a command within the pod', () => {
            let fetched = kubernetes.get("Pod", name, ns)
            expect(pod.metadata.uid, 'created and fetched uids').to.equal(fetched.metadata.uid)

            let greeting = 'hello xk6-kubernetes'
            let exec = {
                pod: name,
                container: fetched.spec.containers[0].name,
                command: ["echo", greeting]
            }
            let result = helpers.executeInPod(exec)
            const stdout = String.fromCharCode(...result.stdout)
            const stderr = String.fromCharCode(...result.stderr)
            expect(stdout, 'execution result').to.contain(greeting)
            expect(stderr, 'execution error').to.be.empty
        })

        describe('Remove our pods to cleanup', () => {
            kubernetes.delete("Pod", name, ns)
        })
    })

    describe('YAML-based resources', () => {
        let yamlObject = load(yaml)
        const name = yamlObject.metadata.name
        const ns = yamlObject.metadata.namespace

        describe('Create our pod using the YAML definition', () => {
            kubernetes.apply(yaml)
            let created = kubernetes.get("Pod", name, ns)
            expect(created.metadata, 'new pod').to.have.property('uid')
        })

        describe('Update our Pod with a modified YAML definition', () => {
            const newValue = "busybox:1.35.0"
            yamlObject.spec.containers[0].image = newValue
            let newYaml = dump(yamlObject)

            kubernetes.apply(newYaml)
            let updated = kubernetes.get("Pod", name, ns)
            expect(updated.spec.containers[0].image, 'changed value').to.be.equal(newValue)
        })

        describe('Remove our pod to cleanup', () => {
            kubernetes.delete("Pod", name, ns)
        })
    })

}
