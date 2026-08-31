#!/usr/bin/env python3
"""Sync every non-English locale with en.js.

For each locale under web/src/shared/locales/ this script:
  1. computes the keys present in en.js but missing from the locale,
  2. translates them via `pi` (one batched call per locale, with a few of the
     locale's existing strings as register/style reference),
  3. regenerates the locale file by walking en.js line-by-line — comments,
     blank lines, and key order all mirror en.js — using existing translations
     where the locale already has them and the fresh ones for the gaps.

Keys the locale has but en.js no longer does are dropped (en.js is the source
of truth). Placeholders like {count} are preserved verbatim; a translation
that loses a placeholder is rejected and the English string kept as fallback.

Usage: python3 scripts/sync_locale_keys.py [code ...]   (default: all locales)
"""
from __future__ import annotations

import json
import re
import subprocess
import sys
from pathlib import Path

REPO = Path(__file__).resolve().parents[1]
LOCALES_DIR = REPO / "web" / "src" / "shared" / "locales"

LANGS = [
    ("de", "Deutsch (German)"),
    ("es", "Español (Spanish)"),
    ("fil", "Filipino"),
    ("fr", "Français (French)"),
    ("id", "Bahasa Indonesia (Indonesian)"),
    ("ja", "日本語 (Japanese)"),
    ("km", "ខ្មែរ (Khmer)"),
    ("lo", "ລາວ (Lao)"),
    ("ms", "Bahasa Melayu (Malay)"),
    ("my", "မြန်မာ (Burmese)"),
    ("th", "ไทย (Thai)"),
    ("vi", "Tiếng Việt (Vietnamese)"),
    ("zh", "中文 (Simplified Chinese)"),
]

BATCH_SIZE = 40
KEY_LINE = re.compile(r"^  '((?:[^'\\]|\\.)+)':")
PLACEHOLDER = re.compile(r"\{[a-zA-Z0-9_]+\}")


def load_locale(path: Path) -> dict[str, str]:
    """Import the locale module via node and return its key/value map."""
    out = subprocess.run(
        [
            "node",
            "--input-type=module",
            "-e",
            "const m = await import(process.argv[1]);"
            "console.log(JSON.stringify(m.default));",
            str(path),
        ],
        capture_output=True,
        text=True,
        check=True,
    )
    return json.loads(out.stdout)


def js_quote(s: str) -> str:
    return "'" + s.replace("\\", "\\\\").replace("'", "\\'") + "'"


def translate_batch(
    lang_name: str, batch: dict[str, str], reference: dict[str, str]
) -> dict[str, str]:
    prompt = (
        f"Translate the VALUES of this JSON object from English to {lang_name} "
        "for a web UI. Rules:\n"
        "- Keep every key exactly as-is; translate only values.\n"
        "- Preserve placeholders like {count}, {name}, {size} verbatim.\n"
        "- Match the tone/register of these existing translations: "
        + json.dumps(reference, ensure_ascii=False)
        + "\n- Reply with ONLY the translated JSON object, no fences, no prose.\n\n"
        + json.dumps(batch, ensure_ascii=False)
    )
    result = subprocess.run(
        ["pi", "-p", "--no-session", prompt],
        capture_output=True,
        text=True,
        timeout=600,
    )
    if result.returncode != 0:
        raise RuntimeError(f"pi failed (exit {result.returncode}): {result.stderr.strip()}")
    text = result.stdout.strip()
    match = re.search(r"\{.*\}", text, re.DOTALL)
    if not match:
        raise ValueError(f"no JSON object in pi output: {text[:200]}")
    return json.loads(match.group(0))


def sync_locale(code: str, lang_name: str, en_lines: list[str], en_map: dict[str, str]) -> None:
    path = LOCALES_DIR / f"{code}.js"
    existing = load_locale(path)
    missing = {k: v for k, v in en_map.items() if k not in existing}
    extra = [k for k in existing if k not in en_map]

    translated: dict[str, str] = {}
    if missing:
        # A few existing translations ground the model in the locale's register.
        reference = dict(list(existing.items())[:8])
        items = list(missing.items())
        for i in range(0, len(items), BATCH_SIZE):
            batch = dict(items[i : i + BATCH_SIZE])
            print(f"[{code}] translating {len(batch)} keys…", flush=True)
            translated.update(translate_batch(lang_name, batch, reference))

    # Reject translations that dropped a placeholder; fall back to English.
    fallbacks = 0
    for key, en_value in missing.items():
        value = translated.get(key)
        if not isinstance(value, str) or not value.strip() or set(
            PLACEHOLDER.findall(en_value)
        ) - set(PLACEHOLDER.findall(value)):
            translated[key] = en_value
            fallbacks += 1

    # First line of the existing file is the locale's own header comment; the
    # body mirrors en.js from its `export default {` line onward (en's own
    # multi-line header comment is skipped).
    header = path.read_text(encoding="utf-8").splitlines()[0]
    body_start = en_lines.index("export default {")

    out: list[str] = [header]
    i = body_start
    while i < len(en_lines):
        line = en_lines[i]
        m = KEY_LINE.match(line)
        if not m:
            out.append(line)
            i += 1
            continue
        key = m.group(1).replace("\\'", "'")
        value = existing.get(key) or translated.get(key) or en_map[key]
        out.append(f"  {js_quote(key)}: {js_quote(value)},")
        # Skip prettier-wrapped continuation lines of this entry (the value
        # moved to its own line(s)); the entry ends at the line closing with a
        # comma. A key line that already ends with ',' is fully consumed.
        while not en_lines[i].rstrip().endswith(","):
            i += 1
        i += 1
    path.write_text("\n".join(out) + "\n", encoding="utf-8")

    dropped = f", dropped {len(extra)} stale" if extra else ""
    fb = f", {fallbacks} English fallbacks" if fallbacks else ""
    print(f"[{code}] wrote {len(missing)} new keys{dropped}{fb}", flush=True)


def main() -> int:
    only = set(sys.argv[1:])
    en_path = LOCALES_DIR / "en.js"
    en_lines = en_path.read_text(encoding="utf-8").splitlines()
    en_map = load_locale(en_path)
    for code, lang_name in LANGS:
        if only and code not in only:
            continue
        sync_locale(code, lang_name, en_lines, en_map)
    return 0


if __name__ == "__main__":
    sys.exit(main())
