import { Kubernetes } from 'k6/x/kubernetes';
import { describe, expect } from 'https://jslib.k6.io/k6chaijs/4.3.4.3/index.js';
import { load, dump } from 'https://cdn.jsdelivr.net/npm/js-yaml@4.1.0/dist/js-yaml.mjs';

let json = {
    apiVersion: "v1",
    kind: "PersistentVolume",
    metadata: {
        name: "json-pv",
    },
    spec: {
        storageClassName: "manual",
        capacity: {
            storage: "1Mi",
        },
        accessModes: [
            "ReadWriteOnce"
        ],
        hostPath: {
            path: "/tmp/k3dvol",
        },
    }
}

let yaml = `
apiVersion: v1
kind: PersistentVolume
metadata:
  name: yaml-pv
spec:
  storageClassName: "manual"
  capacity: 
    storage: 1Mi
  accessModes:
   - ReadWriteOnce
  hostPath:
    path: /tmp/k3dvol
`

export default function () {
    const kubernetes = new Kubernetes();

    describe('JSON-based resources', () => {
        const name = json.metadata.name

        let created

        describe('Create our PersistentVolume using the JSON definition', () => {
            created = kubernetes.create(json)
            expect(created.metadata, 'new persistentvolume').to.have.property('uid')
        })

        describe('Retrieve our PersistentVolume by name', () => {
            let fetched = kubernetes.get("PersistentVolume", name)
            expect(created.metadata.uid, 'created and fetched uids').to.equal(fetched.metadata.uid)
        })

        describe('Update our PersistentVolume with a modified JSON definition', () => {
            const newValue = "10Mi"
            json.spec.capacity.storage = newValue

            kubernetes.update(json)
            let updated = kubernetes.get("PersistentVolume", name)
            expect(updated.spec.capacity.storage, 'changed value').to.be.equal(newValue)
        })

        describe('Remove our PersistentVolume to cleanup', () => {
            kubernetes.delete("PersistentVolume", name)
        })
    })

    describe('YAML-based resources', () => {
        let yamlObject = load(yaml)
        const name = yamlObject.metadata.name

        describe('Create our PersistentVolume using the YAML definition', () => {
            kubernetes.apply(yaml)
            let created = kubernetes.get("PersistentVolume", name)
            expect(created.metadata, 'new persistentvolume').to.have.property('uid')
        })

        describe('Update our PersistentVolume with a modified YAML definition', () => {
            const newValue = "10Mi"
            yamlObject.spec.capacity.storage = newValue
            let newYaml = dump(yamlObject)

            kubernetes.apply(newYaml)
            let updated = kubernetes.get("PersistentVolume", name)
            expect(updated.spec.capacity.storage, 'changed value').to.be.equal(newValue)
        })

        describe('Remove our PersistentVolume to cleanup', () => {
            kubernetes.delete("PersistentVolume", name)
        })
    })

}
