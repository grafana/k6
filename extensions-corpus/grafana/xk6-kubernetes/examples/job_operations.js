import { Kubernetes } from 'k6/x/kubernetes';
import { describe, expect } from 'https://jslib.k6.io/k6chaijs/4.3.4.3/index.js';
import { load } from 'https://cdn.jsdelivr.net/npm/js-yaml@4.1.0/dist/js-yaml.mjs';

let json = {
    apiVersion: "batch/v1",
    kind: "Job",
    metadata: {
        name:      "json-job",
        namespace: "default",
    },
    spec: {
        ttlSecondsAfterFinished: 30,
        template: {
            spec: {
                containers: [
                    {
                        name: "myjob",
                        image: "perl:5.34.0",
                        command: ["perl", "-Mbignum=bpi", "-wle", "print bpi(2000)"],
                    },
                ],
                restartPolicy: "Never",
            }
        },
        backoffLimit: 4,
    }
}

let yaml = `
apiVersion: batch/v1
kind: Job
metadata:
  name: yaml-job
  namespace: default
spec:
  ttlSecondsAfterFinished: 30
  suspend: false
  template:
    spec:
      containers:
      - name: myjob
        image: perl:5.34.0
        command: ["perl",  "-Mbignum=bpi", "-wle", "print bpi(2000)"]
      restartPolicy: Never
  backoffLimit: 4
`

export default function () {
    const kubernetes = new Kubernetes();

    describe('JSON-based resources', () => {
        const name = json.metadata.name
        const ns = json.metadata.namespace
        const helpers = kubernetes.helpers(ns)

        let job

        describe('Create our job using the JSON definition and wait until completed', () => {
            job = kubernetes.create(json)
            expect(job.metadata, 'new job').to.have.property('uid')

            let timeout = 10
            expect(helpers.waitJobCompleted(name, timeout), `job completion within ${timeout}s`).to.be.true

            let fetched = kubernetes.get("Job.batch", name, ns)
            expect(job.metadata.uid, 'created and fetched uids').to.equal(fetched.metadata.uid)
        })

        describe('Retrieve all available jobs', () => {
            expect(kubernetes.list("Job.batch", ns).length, 'total jobs').to.be.at.least(1)
        })

        describe('Remove our jobs to cleanup', () => {
            kubernetes.delete("Job.batch", name, ns)
        })
    })

    describe('YAML-based resources', () => {
        let yamlObject = load(yaml)
        const name = yamlObject.metadata.name
        const ns = yamlObject.metadata.namespace

        describe('Create our job using the YAML definition', () => {
            kubernetes.apply(yaml)
            let created = kubernetes.get("Job.batch", name, ns)
            expect(created.metadata, 'new job').to.have.property('uid')
        })
    })

}
