// Build the release pair together, then load both exact images for verification.
// Publishing remains a separate, gated workflow step.
target "app" {
  context    = "."
  dockerfile = "Dockerfile"
  platforms  = ["linux/amd64"]
  tags       = ["novelreader:e2e"]
  cache-from = ["type=gha,scope=novelreader"]
  cache-to   = ["type=gha,mode=max,scope=novelreader"]
  output     = ["type=docker"]
  attest     = ["type=provenance,disabled=true", "type=sbom,disabled=true"]
}

target "worker" {
  context    = "./webview-worker"
  dockerfile = "Dockerfile"
  platforms  = ["linux/amd64"]
  tags       = ["novelreader-webview:e2e"]
  // Resolve fresh Patchright and Chrome; verify these bytes before promotion.
  no-cache   = true
  output     = ["type=docker"]
  attest     = ["type=provenance,disabled=true", "type=sbom,disabled=true"]
}
