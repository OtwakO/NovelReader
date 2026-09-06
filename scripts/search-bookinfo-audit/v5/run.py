"""Explicit local live audit. Private raw evidence is never printed or tracked."""
import argparse
import collections
import concurrent.futures
import hashlib
import json
import os
from pathlib import Path
import subprocess

parser = argparse.ArgumentParser()
parser.add_argument("phase", choices=("initial", "replay", "scrutiny"))
parser.add_argument("--webview", default="")
parser.add_argument("--run", default="", help="Named observation set beneath the private audit directory")
args = parser.parse_args()
if args.run and (Path(args.run).name != args.run or args.run in (".", "..")):
    parser.error("--run must be a single directory name")
root = Path("test-booksources/engine-audit")
run_root = root / args.run
manifest = json.loads((root / "manifest.json").read_text())
if hashlib.sha256((root / "sources.json").read_bytes()).hexdigest() != manifest["frozenSHA256"]:
    raise ValueError("Frozen source file changed")
indices = list(range(len(manifest["selection"])))
if args.phase != "initial":
    indices = []
    for index in range(len(manifest["selection"])):
        prior = json.loads((run_root / "initial" / f"{index}.json").read_text())
        failed = (prior.get("infrastructureError") or prior.get("searchError") or not prior.get("results")
                  or not prior.get("detailAttempted") or prior.get("detailError"))
        results = prior.get("results") or []
        suspicious = (not any(manifest["query"] in result.get("name", "") for result in results)
                      or (len(results) > 1 and len({result.get("bookUrl") for result in results}) == 1))
        if (args.phase == "replay" and failed) or (args.phase == "scrutiny" and not failed and suspicious):
            indices.append(index)
os.umask(0o077)
output = run_root / args.phase
output.mkdir(parents=True, exist_ok=False)

def run(index):
    command = [str(root / "engine-audit"), "-sources", str(root / "sources.json"),
               "-index", str(index), "-query", manifest["query"]]
    if args.webview:
        command.extend(["-webview", args.webview])
    with (output / f"{index}.json").open("w") as stdout, (output / f"{index}.log").open("w") as stderr:
        try:
            result = subprocess.run(command, stdout=stdout, stderr=stderr, timeout=45)
            failure = "process_exit" if result.returncode else ""
        except subprocess.TimeoutExpired:
            failure = "process_timeout"
    if failure:
        (output / f"{index}.json").write_text(json.dumps({"index": index, "infrastructureError": failure}))
    record = json.loads((output / f"{index}.json").read_text())
    if record.get("infrastructureError"):
        return "audit_infrastructure"
    if record["searchError"]:
        return "search_error_unclassified"
    if not record["results"]:
        return "empty_unclassified"
    if not record["detailAttempted"]:
        return "no_credible_result"
    return "detail_error_unclassified" if record["detailError"] else "search_and_detail_returned"

with concurrent.futures.ThreadPoolExecutor(max_workers=4 if args.phase == "initial" else 1) as executor:
    counts = collections.Counter(executor.map(run, indices))
print(json.dumps({"run": args.run, "phase": args.phase, "count": len(indices), "observations_not_final_classifications": dict(counts)}))
