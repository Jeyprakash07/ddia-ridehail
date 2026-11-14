# Runs the simulator inside the cluster as a Job and prints its report.
#
#   .\scripts\load.ps1                                  # 10,000 drivers for 60 s
#   .\scripts\load.ps1 -Drivers 40000 -Duration 2m
#   .\scripts\load.ps1 -StallAt 20s -StallFor 2s        # freeze ingest mid-run
param(
    [int]$Drivers = 10000,
    [string]$PingInterval = "4s",
    [string]$Duration = "60s",
    [int]$Workers = 64,
    [string]$StallAt = "0s",
    [string]$StallFor = "2s"
)

function Assert-Ok([string]$What) {
    if ($LASTEXITCODE -ne 0) { throw "$What failed (exit code $LASTEXITCODE)" }
}

# Use whatever image ingest is running, so both ends come from the same build.
$image = kubectl -n ridehail get deployment ingest -o "jsonpath={.spec.template.spec.containers[0].image}"
Assert-Ok "image lookup"

$name = "sim-" + (Get-Date -Format "HHmmss")
$job = @"
apiVersion: batch/v1
kind: Job
metadata:
  name: $name
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
          image: $image
          imagePullPolicy: IfNotPresent
          command: ["/usr/local/bin/simulator"]
          args:
            - -target=http://ingest:8080
            - -drivers=$Drivers
            - -ping-interval=$PingInterval
            - -duration=$Duration
            - -workers=$Workers
            - -stall-at=$StallAt
            - -stall-for=$StallFor
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
"@

$job | kubectl create -f -
Assert-Ok "job creation"

kubectl -n ridehail wait --for=condition=ready pod -l "job-name=$name" --timeout=90s
Assert-Ok "simulator start"

kubectl -n ridehail logs -f "job/$name"
