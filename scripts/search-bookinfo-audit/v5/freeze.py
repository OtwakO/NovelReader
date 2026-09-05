"""Freeze the user-approved corpus sample; all source-bearing output stays ignored."""
import hashlib
import json
from pathlib import Path

corpus_path = Path("test-booksources/new_test_booksource.json")
output = Path("test-booksources/engine-audit")
seed = "NovelReader-engine-audit-v1"
raw = corpus_path.read_bytes()
text = raw.decode("utf-8-sig")
decoder = json.JSONDecoder()
items = []
position = text.index("[") + 1
while True:
    while text[position].isspace() or text[position] == ",":
        position += 1
    if text[position] == "]":
        break
    item, end = decoder.raw_decode(text, position)
    items.append((item, text[position:end]))
    position = end

eligible = []
for index, (source, fragment) in enumerate(items):
    if (source.get("enabled") is False or int(source.get("bookSourceType", 0)) != 0
            or not str(source.get("searchUrl") or "").strip() or not source.get("ruleSearch")):
        continue
    rank = hashlib.sha256(f"{seed}\0{index}\0{source.get('bookSourceUrl', '')}".encode()).hexdigest()
    eligible.append((rank, index, fragment))
selected = sorted(eligible)[:50]
if len(selected) != 50:
    raise ValueError("Expected at least 50 eligible sources")
output.mkdir(parents=True, exist_ok=True)
frozen = "[\n" + ",\n".join(fragment for _, _, fragment in selected) + "\n]\n"
manifest = {
    "seed": seed, "query": "凡人修仙传", "corpus": str(corpus_path),
    "corpusSHA256": hashlib.sha256(raw).hexdigest(), "eligible": len(eligible),
    "frozenSHA256": hashlib.sha256(frozen.encode()).hexdigest(),
    "selection": [{"rawIndex": index, "rank": rank,
                   "definitionSHA256": hashlib.sha256(fragment.encode()).hexdigest()}
                  for rank, index, fragment in selected],
}
(output / "sources.json").write_text(frozen, encoding="utf-8")
(output / "manifest.json").write_text(json.dumps(manifest, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
print(f"Frozen {len(selected)}/{len(eligible)} eligible sources; raw definition text preserved; private output: {output}")
