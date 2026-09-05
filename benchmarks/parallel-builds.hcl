// Benchmark-only targets. No image push or cache export.
target "app" {
  context    = "."
  dockerfile = "Dockerfile"
  platforms  = ["linux/amd64"]
  tags       = ["novelreader:benchmark-app"]
  cache-from = ["type=gha,scope=novelreader"]
  output     = ["type=docker"]
  attest     = ["type=provenance,disabled=true", "type=sbom,disabled=true"]
}

target "worker" {
  context    = "./webview-worker"
  dockerfile = "Dockerfile"
  platforms  = ["linux/amd64"]
  tags       = ["novelreader-webview:benchmark-worker"]
  no-cache   = true
  output     = ["type=docker"]
  attest     = ["type=provenance,disabled=true", "type=sbom,disabled=true"]
}
