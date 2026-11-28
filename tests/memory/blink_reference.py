"""Generate token IDs and normalized vectors using the official, local-only reference.

Usage: python tests/memory/blink_reference.py [verified-bundle-directory]
With no argument, regenerate the small deterministic fixture used by make test.
"""
import base64
import hashlib
from importlib.metadata import version
import json
from pathlib import Path
import sys
import tempfile

import numpy as np
from model2vec import StaticModel
from safetensors.numpy import save_file
from tokenizers import AddedToken, Tokenizer, models, normalizers, pre_tokenizers

ROOT = Path(__file__).resolve().parents[2]
OUT = ROOT / "internal/blink/testdata"
VERSIONS = {"model2vec": "0.11.0", "model2vec-rs": "0.5.1", "tokenizers": "0.23.3", "numpy": "2.4.4"}
for package, pin in VERSIONS.items():
    if version(package) != pin:
        raise RuntimeError(f"Install {package}=={pin} to regenerate these goldens")

# Include extra tokens after the 512th and unknowns before a finite token limit.
CASES = [
    ("english", "Concurrent updates keep the queue busy.", None),
    ("portuguese", "Ação, São Paulo: decisões e café.", None),
    ("decomposed", "Ac\u0327a\u0303o, Sa\u0303o Paulo: deciso\u0303es e cafe\u0301.", None),
    ("code", "src/cache_v2/FlushLeaseJournal.go::Put_ID(x86-64)", None),
    ("special", "this[UNK]queue[CLS][MASK][unk]next", None),
    ("added_words", "東京東京都!目录：東京都ward MiXeDaMiXeD mixed", None),
    ("normalization", "A\u0001B\u200dC\ue000 Ｋ İ Σ 中한\ufffd? $ + ✨", None),
    ("category_boundaries", "a\U0001cf00b c\u0378d e\U00011fffz f\u1734g q\u111c9v", None),
    ("long_word", "ação" * 54 + "x", None),
    ("long_sequence", "queue " * 550 + "recovery " * 90, None),
    ("unknown_before_limit", "\U0010ffff ! ! ! ! queue recovery", 5),
    ("character_limit", "                 queue", 2),
    ("empty", "", None),
    ("unknown", "\U0010ffff [UNK]", None),
]


def synthetic_bundle(path):
    specials = ["[PAD]", "[UNK]", "[CLS]", "[SEP]", "[MASK]"]
    words = "concurrent updates keep the queue busy acao sao paulo decisoes e cafe src cache v2 flush lease journal go put id x86 64 this next abc k i σ 中 recovery".split()
    pieces = list("abcdefghijklmnopqrstuvwxyz0123456789.,:/_()-?[]!$+")
    vocab = list(dict.fromkeys(specials + words + pieces + ["##" + c for c in pieces]))
    tokenizer = Tokenizer(models.WordPiece({w: i for i, w in enumerate(vocab)}, unk_token="[UNK]"))
    tokenizer.normalizer = normalizers.BertNormalizer(clean_text=True, handle_chinese_chars=False, strip_accents=True, lowercase=True)
    tokenizer.pre_tokenizer = pre_tokenizers.Sequence([pre_tokenizers.BertPreTokenizer(), pre_tokenizers.FixedLength(32)])
    tokenizer.add_special_tokens(specials)
    tokenizer.add_tokens([AddedToken(text, normalized=False) for text in ("東京", "東京都", "MiXeD")])
    tokenizer.save(str(path / "tokenizer.json"), pretty=True)
    (path / "config.json").write_text(json.dumps({"normalize": True, "embedding_dtype": "int8", "max_length": None}))
    i, j = np.indices((tokenizer.get_vocab_size(), 256))
    table = ((i * 17 + j * 7) % 251 - 125).astype(np.int8)
    save_file({"embeddings": table}, str(path / "model.safetensors"))


def generate(path, output):
    model = StaticModel.from_pretrained(path)
    cases = []
    default_length = model.max_length
    base_lengths = sorted(len(w) for w in model.tokenizer.get_vocab(with_added_tokens=False))
    median = base_lengths[len(base_lengths) // 2]
    for name, text, limit in CASES:
        # encode uses its configured limit unless explicitly overridden.
        max_length = default_length if limit is None else limit
        model.max_length = max_length
        probe = text if max_length is None else text[:max_length * median]
        ids = model.tokenize([probe])[0]
        vector = model.encode(text)
        cases.append({"name": name, "text": text, "max_length": limit, "ids": ids,
                      "f32le": base64.b64encode(vector.astype("<f4").tobytes()).decode()})
    # Keep each expected token sequence together in the checked-in diff.
    blocks = []
    for case in cases:
        fields = [f'      "{key}": {json.dumps(value, ensure_ascii=False)}' for key, value in case.items()]
        blocks.append("    {\n" + ",\n".join(fields) + "\n    }")
    output.write_text('{\n  "versions": ' + json.dumps(VERSIONS) + ',\n  "cases": [\n'
                      + ",\n".join(blocks) + "\n  ]\n}\n")


if __name__ == "__main__":
    if len(sys.argv) > 1:
        bundle = Path(sys.argv[1]).resolve()
        for line in (ROOT / "scripts/blink.sha256").read_text().splitlines():
            checksum, name = line.split()
            if hashlib.sha256((bundle / name).read_bytes()).hexdigest() != checksum:
                raise ValueError(f"{name}: incorrect release checksum")
        generate(bundle, OUT / "blink.json")
    else:
        with tempfile.TemporaryDirectory() as d:
            bundle = Path(d)
            synthetic_bundle(bundle)
            (OUT / "tokenizer.json").write_bytes((bundle / "tokenizer.json").read_bytes())
            generate(bundle, OUT / "synthetic.json")
