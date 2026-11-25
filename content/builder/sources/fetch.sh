#!/usr/bin/env bash
# fetch.sh DIR — the Wikisource pages content/builder/make.py reads, as
# rendered HTML (h1…h8.json), converted to text (h1…h8.txt) by h2t.py.
# Then: python content/builder/make.py DIR content/builder
set -euo pipefail
d=${1:?a directory}; mkdir -p "$d"
here=$(dirname "$0")
ua='esperanto-kurso-builder/1 (course content research)'
i=0
for t in "Fundamento de Esperanto/Antaŭparolo" \
         "Lingvaj Respondoj (1925)/Letero pri la deveno de Esperanto" \
         "Fundamenta Krestomatio (1903)/Esenco kaj estonteco de la ideo de lingvo internacia" \
         "Hamleto/Akto III" \
         "Bulonja deklaracio" \
         "La Revizoro/Akto unua" \
         "Fundamenta Krestomatio (1903)/La espero" \
         "Sinjoro Tadeo/Libro I"; do
  i=$((i+1))
  e=$(python -c "import urllib.parse,sys;print(urllib.parse.quote(sys.argv[1].replace(' ','_')))" "$t")
  curl -sS -A "$ua" "https://eo.wikisource.org/w/api.php?action=parse&format=json&prop=text&page=$e" -o "$d/h$i.json"
done
# Fundamenta Krestomatio (1903): science and essays
for pair in "termitoj|La loĝejoj de la termitoj" \
            "einstein|Al la historio de la provoj de lingvoj tutmondaj de Leibnitz ĝis la nuna tempo" \
            "konjunktivito|Kronika katara konjunktivito" \
            "sunhorlogxo|La sunhorloĝo en Dijon"; do
  e=$(python -c "import urllib.parse,sys;print(urllib.parse.quote(('Fundamenta Krestomatio (1903)/'+sys.argv[1]).replace(' ','_')))" "${pair#*|}")
  curl -sS -A "$ua" "https://eo.wikisource.org/w/api.php?action=parse&format=json&prop=text&page=$e" -o "$d/kr-${pair%%|*}.json"
done
python -I "$here/h2t.py" "$d"/h*.json "$d"/kr-*.json
