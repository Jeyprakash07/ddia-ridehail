# Builds the image, loads it into the cluster and applies the manifests.
# Run it again after any code change.
#
#   .\scripts\deploy.ps1

function Assert-Ok([string]$What) {
    if ($LASTEXITCODE -ne 0) { throw "$What failed (exit code $LASTEXITCODE)" }
}

$root = Split-Path -Parent $PSScriptRoot
# A fresh tag per build, so the cluster can never run a stale image by mistake.
$tag = "ridehail:" + (Get-Date -Format "yyyyMMdd-HHmmss")

docker build -t $tag $root
Assert-Ok "docker build"

minikube image load $tag
Assert-Ok "minikube image load"

kubectl apply -f "$root\deploy\namespace.yaml"
Assert-Ok "namespace"

# The database password is generated once and exists only inside the cluster.
$existing = kubectl -n ridehail get secret postgres --ignore-not-found -o name
Assert-Ok "secret lookup"
if (-not $existing) {
    $bytes = New-Object byte[] 24
    [Security.Cryptography.RandomNumberGenerator]::Create().GetBytes($bytes)
    $password = -join ($bytes | ForEach-Object { $_.ToString("x2") })
    kubectl -n ridehail create secret generic postgres "--from-literal=password=$password"
    Assert-Ok "secret creation"
}

kubectl apply -f "$root\deploy\postgres.yaml"
Assert-Ok "postgres manifest"

(Get-Content "$root\deploy\ingest.yaml" -Raw).Replace("ridehail:dev", $tag) | kubectl apply -f -
Assert-Ok "ingest manifest"

kubectl -n ridehail rollout status statefulset/postgres --timeout=300s
Assert-Ok "postgres rollout"
kubectl -n ridehail rollout status deployment/ingest --timeout=180s
Assert-Ok "ingest rollout"

Write-Host ""
Write-Host "Deployed $tag"
