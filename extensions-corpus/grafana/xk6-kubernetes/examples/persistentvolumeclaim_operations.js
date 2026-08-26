import { Kubernetes } from 'k6/x/kubernetes';
import { describe, expect } from 'https://jslib.k6.io/k6chaijs/4.3.4.3/index.js';
import { load } from 'https://cdn.jsdelivr.net/npm/js-yaml@4.1.0/dist/js-yaml.mjs';

let json = {
    apiVersion: "v1",
    kind: "PersistentVolumeClaim",
    metadata: {
        name: "json-pvc",
        namespace: "default",
    },
    spec: {
        storageClassName: "",
        accessModes: ["ReadWriteMany"],
        resources: {
            requests: {
                storage: "10Mi",
            }
        }
    }
}

let yaml = `
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: yaml-pvc
  namespace: default
spec:
  storageClassName: ""
  accessModes:
  - ReadWriteMany
  resources:
    requests:
      storage: "10Mi"
`

export default function () {
    const kubernetes = new Kubernetes();

    describe('JSON-based resources', () => {
        const name = json.metadata.name
        const ns = json.metadata.namespace

        let pvc

        describe('Create our PersistentVolumeClaim using the JSON definition', () => {
            pvc = kubernetes.create(json)
            expect(pvc.metadata, 'new persistentvolumeclaim').to.have.property('uid')
        })

        describe('Retrieve all available PersistentVolumeClaims', () => {
            expect(kubernetes.list("PersistentVolumeClaim", ns).length, 'total persistentvolumeclaims').to.be.at.least(1)
        })

        describe('Retrieve our PersistentVolumeClaim by name', () => {
            let fetched = kubernetes.get("PersistentVolumeClaim", name, ns)
            expect(pvc.metadata.uid, 'created and fetched uids').to.equal(fetched.metadata.uid)
        })

        describe('Remove our PersistentVolumeClaim to cleanup', () => {
            kubernetes.delete("PersistentVolumeClaim", name, ns)
        })
    })

    describe('YAML-based resources', () => {
        let yamlObject = load(yaml)
        const name = yamlObject.metadata.name
        const ns = yamlObject.metadata.namespace

        describe('Create our PersistentVolumeClaim using the YAML definition', () => {
            kubernetes.apply(yaml)
            let created = kubernetes.get("PersistentVolumeClaim", name, ns)
            expect(created.metadata, 'new persistentvolumeclaim').to.have.property('uid')
        })

        describe('Remove our PersistentVolumeClaim to cleanup', () => {
            kubernetes.delete("PersistentVolumeClaim", name, ns)
        })
    })

}
