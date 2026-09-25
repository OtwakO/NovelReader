"""Publication contract tests: exercise the real shell against a synthetic registry."""
import json
import os
from pathlib import Path
import subprocess
import tempfile
import unittest

SCRIPT = Path(__file__).with_name("publish-images.sh").resolve()
DOCKER = r'''#!/usr/bin/env python3
import json, os, sys
from pathlib import Path
args = sys.argv[1:]
root = Path(os.environ["REGISTRY"])
with (root / "calls").open("a") as log:
    log.write(json.dumps(args) + "\n")
assert args[:2] == ["buildx", "imagetools"], args
if args[2] == "create":
    tag = args[args.index("--tag") + 1]
    if ":ci-" in tag:
        if os.environ.get("FAIL_CREATE"):
            sys.exit(1)
        manifests = [
            {"digest": ref.split("@")[1], "platform": {"os": "linux", "architecture": arch}}
            for arch, ref in zip(("amd64", "arm64"), [arg for arg in args if "@" in arg])
        ]
        if os.environ.get("WRONG_PLATFORM"):
            manifests[1]["platform"]["architecture"] = "amd64"
        (root / tag.split(":")[0]).write_text(json.dumps({"manifests": manifests}))
elif args[2] == "inspect":
    ref = args[3]
    if ":sha-" in ref:
        if os.environ.get("CONFLICT"):
            print("sha256:" + "9" * 64)
        else:
            sys.exit(1)
    elif "--raw" in args:
        print((root / ref.split("@")[0]).read_text())
    else:
        if os.environ.get("FAIL_INSPECT"):
            sys.exit(1)
        print("sha256:" + ("a" if ref.startswith("app:") else "b") * 64)
else:
    raise AssertionError(args)
'''


class PublicationTest(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        docker = self.root / "docker"
        docker.write_text(DOCKER)
        docker.chmod(0o755)
        self.digests = self.root / "digests"
        self.digests.mkdir()
        for index, name in enumerate(("amd64-app", "arm64-app", "amd64-worker", "arm64-worker"), 1):
            (self.digests / name).write_text("sha256:" + str(index) * 64 + "\n")
        self.env = dict(os.environ, PATH=f"{self.root}:{os.environ['PATH']}", REGISTRY=str(self.root),
                        APP_IMAGE="app", WORKER_IMAGE="worker", GITHUB_RUN_ID="1", GITHUB_RUN_ATTEMPT="1",
                        GITHUB_SHA="f" * 40, GITHUB_EVENT_NAME="push", GITHUB_REF_TYPE="branch",
                        GITHUB_REF_NAME="main", PUBLISH_IMAGES="true")

    def run_publish(self, **env):
        return subprocess.run(["bash", str(SCRIPT), str(self.digests)], env={**self.env, **env},
                              capture_output=True, text=True, timeout=10)

    def published_tags(self):
        calls = self.root / "calls"
        if not calls.exists():
            return []
        return [args[index + 1] for line in calls.read_text().splitlines()
                for args in [json.loads(line)] if args[2] == "create"
                for index, value in enumerate(args) if value == "--tag" and ":ci-" not in args[index + 1]]

    def test_verified_pair_promotes_worker_before_app(self):
        result = self.run_publish()
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(self.published_tags(), ["app:sha-" + "f" * 40, "worker:sha-" + "f" * 40,
                                                "worker:latest", "worker:edge", "app:latest", "app:edge"])

    def test_verification_only_never_promotes(self):
        result = self.run_publish(PUBLISH_IMAGES="false")
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertTrue((self.root / "app").exists())
        self.assertTrue((self.root / "worker").exists())
        self.assertEqual(self.published_tags(), [])

    def test_missing_architecture_blocks_publication(self):
        (self.digests / "arm64-app").unlink()
        self.assertNotEqual(self.run_publish().returncode, 0)
        self.assertEqual(self.published_tags(), [])

    def test_registry_failures_and_wrong_platform_block_publication(self):
        for failure in ("FAIL_CREATE", "FAIL_INSPECT", "WRONG_PLATFORM", "CONFLICT"):
            with self.subTest(failure=failure):
                (self.root / "calls").write_text("")
                result = self.run_publish(**{failure: "1"})
                self.assertNotEqual(result.returncode, 0, result.stdout)
                self.assertEqual(self.published_tags(), [])

    def test_release_and_manual_aliases_are_preserved(self):
        for env, alias in ((dict(GITHUB_REF_TYPE="tag", GITHUB_REF_NAME="v1.0"), "1.0"),
                           (dict(GITHUB_EVENT_NAME="workflow_dispatch"), "manual")):
            with self.subTest(alias=alias):
                (self.root / "calls").write_text("")
                result = self.run_publish(**env)
                self.assertEqual(result.returncode, 0, result.stderr)
                self.assertEqual(self.published_tags()[-2:], ["worker:" + alias, "app:" + alias])
