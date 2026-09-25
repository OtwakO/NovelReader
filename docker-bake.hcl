// Build the release pair together, then load both exact images for verification.
// Publishing remains a separate, gated workflow step.
// CI sets the platform to match its native runner; local builds use their host.
variable "PLATFORM" {
  default = ""
}
variable "CACHE_ARCH" {
  default = "local"
}
target "app" {
  context    = "."
  dockerfile = "Dockerfile"
  platforms  = PLATFORM == "" ? [] : [PLATFORM]
  tags       = ["novelreader:e2e"]
  cache-from = ["type=gha,scope=novelreader-${CACHE_ARCH}"]
  cache-to   = ["type=gha,mode=max,scope=novelreader-${CACHE_ARCH}"]
  output     = ["type=docker"]
  attest     = ["type=provenance,disabled=true", "type=sbom,disabled=true"]
}

target "worker" {
  context    = "./webview-worker"
  dockerfile = "Dockerfile"
  platforms  = PLATFORM == "" ? [] : [PLATFORM]
  tags       = ["novelreader-webview:e2e"]
  // Resolve fresh Patchright and Chrome; verify these bytes before promotion.
  no-cache   = true
  output     = ["type=docker"]
  attest     = ["type=provenance,disabled=true", "type=sbom,disabled=true"]
}
