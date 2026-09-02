# BookSource conformance fixtures

These are small deterministic inputs for automated compatibility tests. They isolate a NovelReader or Legado contract so live-site changes cannot hide regressions.

New fixtures must be synthetic and minimal: include only the rule shape and response data required by the test, use reserved domains or local test servers, and never embed a complete real BookSource, copied source script, credential, cookie, or private endpoint.

- `core/manifest.json` indexes baseline Search, Book Info, TOC, content, JSONPath, XPath, regex, request, cookie, pagination, and WebView-option fixtures.
- Existing Explore fixtures pin historical compatibility cases. They predate the current private-source policy and must not be used as precedent for committing additional raw sources. New Explore coverage should use purpose-built synthetic source objects and companion response fixtures.

A fixture change requires focused deterministic coverage. Do not replace a fixture with a live URL, put dated audit output here, or add a duplicate synthetic case solely to mirror an optional local real-source test.
