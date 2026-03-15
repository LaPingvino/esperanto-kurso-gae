#!/usr/bin/env python
"""
Generate vocab JSON from zagr YAML files for import into esperanto-kurso-gae.
Usage: python gen.py > /tmp/zagr-vocab.json
"""

import os
import sys
import json
import re

# Try yaml, fall back to manual parsing
try:
    import yaml
    HAS_YAML = True
except ImportError:
    HAS_YAML = False


def word_to_slug(word):
    """Convert Esperanto word to ASCII slug fragment."""
    r = str(word)
    for src, dst in [('ĉ','cx'),('Ĉ','cx'),('ĝ','gx'),('Ĝ','gx'),
                     ('ĥ','hx'),('Ĥ','hx'),('ĵ','jx'),('Ĵ','jx'),
                     ('ŝ','sx'),('Ŝ','sx'),('ŭ','ux'),('Ŭ','ux')]:
        r = r.replace(src, dst)
    # Only keep lowercase letters, digits, hyphens
    r = re.sub(r'[^a-z0-9\-]', '', r.lower())
    return r


def parse_yaml_simple(text):
    """Very simple YAML parser for the zagr format (key: value or key: [list])."""
    result = {}
    lines = text.splitlines()
    i = 0
    current_key = None
    current_list = None
    while i < len(lines):
        line = lines[i]
        # Skip comments and empty lines
        stripped = line.strip()
        if not stripped or stripped.startswith('#'):
            i += 1
            continue
        # List item
        if stripped.startswith('- '):
            if current_key is not None:
                val = stripped[2:].strip().strip("'\"")
                if current_list is None:
                    current_list = []
                current_list.append(val)
                result[current_key] = current_list
            i += 1
            continue
        # Key: value or Key:
        if ':' in stripped:
            # Finish any pending list
            current_list = None
            colon = stripped.index(':')
            key = stripped[:colon].strip()
            value = stripped[colon+1:].strip().strip("'\"")
            current_key = key
            if value:
                result[key] = value
            else:
                result[key] = None  # might become a list
        i += 1
    return result


def load_yaml_file(path):
    with open(path, 'r', encoding='utf-8') as f:
        text = f.read()
    if HAS_YAML:
        try:
            data = yaml.safe_load(text)
            return data or {}
        except Exception:
            # Fall back to simple parser on YAML errors
            return parse_yaml_simple(text)
    else:
        return parse_yaml_simple(text)


def translations_to_string(val):
    """Convert a translation value (string or list) to a single string."""
    if val is None:
        return ''
    def clean_one(s):
        s = str(s).strip()
        s = re.sub(r',?\s*词根,?', '', s)
        s = re.sub(r',?\s*kata akar,?', '', s)
        return s.strip(' ,;')

    if isinstance(val, list):
        parts = [clean_one(v) for v in val if v and clean_one(v)]
        return '; '.join(parts)
    return clean_one(val)


def get_category_tag(filename):
    """Extract category tag from filename like 'en_radiko.yml' -> 'radiko'."""
    name = os.path.basename(filename)
    name = re.sub(r'\.yml$', '', name)
    # Remove language prefix (handles zh-tw_radiko -> radiko)
    parts = name.split('_', 1)
    if len(parts) > 1:
        cat = parts[1]
        # Normalize: tago_en_la_semajno -> tago-en-la-semajno
        cat = cat.replace('_', '-')
        return cat
    return 'radiko'


def scan_directory(directory):
    """
    Scan all YAML files in directory.
    Returns: dict mapping eo_key -> {lang: translation_string, ...}
    Also returns: dict mapping eo_key -> set of category tags
    """
    all_defs = {}   # eo_key -> {lang: str}
    all_cats = {}   # eo_key -> set of tags

    files = sorted(os.listdir(directory))
    for fname in files:
        if not fname.endswith('.yml'):
            continue
        path = os.path.join(directory, fname)

        # Parse language code from filename
        # Filenames like: en_radiko.yml, zh-tw_koloro.yml
        name = re.sub(r'\.yml$', '', fname)
        # Split on first _ that's preceded by known lang patterns
        # Language codes can be like 'en', 'zh-tw', 'zh_tw' -> normalize to 'zh-tw'
        # Find the category part: everything after the lang code
        # Strategy: find the last part that's a known category
        known_cats = {'radiko','adverbo','cifero','grava-esprimo','grava_esprimo',
                      'koloro','konjunkcio','monato','prefikso','prepozicio',
                      'pronomo','sezono','sufikso','tabelvorto',
                      'tago-en-la-semajno','tago_en_la_semajno'}
        # Try to extract lang and category
        lang = None
        cat = None
        for known in known_cats:
            kn_under = known.replace('-', '_')
            if name.endswith('_' + kn_under):
                lang = name[:-(len(kn_under)+1)]
                cat = known.replace('_', '-')
                break
        if lang is None:
            # Unknown pattern, skip
            continue
        # Normalize lang: underscores to hyphens (zh_tw -> zh-tw) except that
        # the directory uses 'zh-tw' already
        lang = lang.replace('_', '-')

        data = load_yaml_file(path)
        if not data:
            continue

        for eo_key, val in data.items():
            eo_key = str(eo_key).strip()
            if not eo_key:
                continue
            # Skip single-character keys and proper nouns (start with uppercase)
            if len(eo_key) <= 1 or (eo_key[0].isupper() and not eo_key[0] in 'ĈĜĤĴŜŬ'):
                continue
            eo_key = eo_key.lower()
            trans = translations_to_string(val)
            if not trans:
                continue

            if eo_key not in all_defs:
                all_defs[eo_key] = {}
            if eo_key not in all_cats:
                all_cats[eo_key] = set()

            all_defs[eo_key][lang] = trans
            all_cats[eo_key].add(cat)

    return all_defs, all_cats


def main():
    zagr_all_dir = '/tmp/zagr-all'
    zagr_vortaro_dir = '/tmp/zagr-vortaro'

    # Scan zagr-all (primary source, many languages)
    all_defs, all_cats = scan_directory(zagr_all_dir)

    # Scan zagr-vortaro (may have additional content for de/en/es/fr/nl/pt)
    # These use just 'en.yml' format without category suffix
    for fname in os.listdir(zagr_vortaro_dir):
        if not fname.endswith('.yml'):
            continue
        lang = re.sub(r'\.yml$', '', fname)
        path = os.path.join(zagr_vortaro_dir, fname)
        data = load_yaml_file(path)
        if not data:
            continue
        for eo_key, val in data.items():
            eo_key = str(eo_key).strip()
            if not eo_key:
                continue
            if len(eo_key) <= 1 or (eo_key[0].isupper() and eo_key[0] not in 'ĈĜĤĴŜŬ'):
                continue
            eo_key = eo_key.lower()
            trans = translations_to_string(val)
            if not trans:
                continue
            if eo_key not in all_defs:
                all_defs[eo_key] = {}
            if lang not in all_defs[eo_key]:  # don't overwrite zagr-all
                all_defs[eo_key][lang] = trans
            if eo_key not in all_cats:
                all_cats[eo_key] = set()
            all_cats[eo_key].add('radiko')

    # Generate ContentItem JSON
    items = []
    for eo_key, defs in sorted(all_defs.items()):
        if not defs:
            continue
        slug_suffix = word_to_slug(eo_key)
        if not slug_suffix:
            continue
        slug = 'voc-zagr-' + slug_suffix

        cats = sorted(all_cats.get(eo_key, {'radiko'}))
        tags = ['vortaro', 'zagr'] + cats

        item = {
            "Slug": slug,
            "Type": "vocab",
            "Content": {
                "word": eo_key,
                "definitions": defs
            },
            "Tags": tags,
            "Source": "zagr",
            "Status": "approved",
            "Rating": 1500.0,
            "RD": 200.0,
            "Volatility": 0.06,
            "VoteScore": 0,
            "Version": 1,
            "ImageURL": "",
            "SeriesSlug": "",
            "SeriesOrder": 0,
        }
        items.append(item)

    json.dump(items, sys.stdout, ensure_ascii=False, indent=2)
    print()  # newline at end
    sys.stderr.write("Generated {} vocab items\n".format(len(items)))


if __name__ == '__main__':
    main()
