# Starts the local cluster and its monitoring stack. Safe to run again.
#
#   .\scripts\cluster-up.ps1
#   .\scripts\cluster-up.ps1 -Cpus 6 -Memory 5g
param(
    [int]$Cpus = 8,
    [string]$Memory = "6g",
    [string]$DiskSize = "40g"
)

function Assert-Ok([string]$What) {
    if ($LASTEXITCODE -ne 0) { throw "$What failed (exit code $LASTEXITCODE)" }
}

$root = Split-Path -Parent $PSScriptRoot

minikube start --driver=docker --cpus=$Cpus --memory=$Memory --disk-size=$DiskSize
Assert-Ok "minikube start"

helm repo add prometheus-community https://prometheus-community.github.io/helm-charts --force-update
Assert-Ok "helm repo add"

helm upgrade --install monitoring prometheus-community/kube-prometheus-stack `
    --namespace monitoring --create-namespace `
    --values "$root\deploy\monitoring-values.yaml" `
    --wait --timeout 10m
Assert-Ok "helm install of kube-prometheus-stack"

Write-Host ""
Write-Host "Cluster is up. To open Grafana on http://localhost:3000 run:"
Write-Host "  kubectl -n monitoring port-forward svc/monitoring-grafana 3000:80"
