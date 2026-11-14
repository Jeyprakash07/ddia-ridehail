#!/usr/bin/env bash
# Runs the simulator inside the cluster as a Job and prints its report.
#
#   scripts/load.sh                                   # 10,000 drivers for 60 s
#   DRIVERS=40000 DURATION=2m scripts/load.sh
#   STALL_AT=20s STALL_FOR=2s scripts/load.sh         # freeze ingest mid-run
set -euo pipefail

DRIVERS="${DRIVERS:-10000}"
PING_INTERVAL="${PING_INTERVAL:-4s}"
DURATION="${DURATION:-60s}"
WORKERS="${WORKERS:-64}"
STALL_AT="${STALL_AT:-0s}"
STALL_FOR="${STALL_FOR:-2s}"

# Use whatever image ingest is running, so both ends come from the same build.
IMAGE="$(kubectl -n ridehail get deployment ingest -o 'jsonpath={.spec.template.spec.containers[0].image}')"
NAME="sim-$(date +%H%M%S)"

kubectl create -f - <<EOF
apiVersion: batch/v1
kind: Job
metadata:
  name: $NAME
  namespace: ridehail
  labels:
    app: simulator
spec:
  backoffLimit: 0
  ttlSecondsAfterFinished: 3600
  template:
    metadata:
      labels:
        app: simulator
    spec:
      restartPolicy: Never
      containers:
        - name: simulator
          image: $IMAGE
          imagePullPolicy: IfNotPresent
          command: ["/usr/local/bin/simulator"]
          args:
            - -target=http://ingest:8080
            - -drivers=$DRIVERS
            - -ping-interval=$PING_INTERVAL
            - -duration=$DURATION
            - -workers=$WORKERS
            - -stall-at=$STALL_AT
            - -stall-for=$STALL_FOR
          env:
            - name: GOMAXPROCS
              valueFrom:
                resourceFieldRef:
                  resource: limits.cpu
          # Capped so the load generator cannot starve the system under test.
          resources:
            requests:
              cpu: "1"
              memory: 128Mi
            limits:
              cpu: "2"
              memory: 512Mi
EOF

kubectl -n ridehail wait --for=condition=ready pod -l "job-name=$NAME" --timeout=90s
kubectl -n ridehail logs -f "job/$NAME"
