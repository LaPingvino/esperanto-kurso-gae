# Esperanto-kurso

Interaga lingvlerneja platformo por Esperanto, piedfunkcianta ĉe **[esperanto-kurso.net](https://esperanto-kurso.net)**.

## Kio ĝi estas

Mem-gastigebla, adaptia lernplatformo konstruita per Go sur Google App Engine. Ĝi havas:

- **Adaptia malfacileco** — Glicko-2 taksadoj kalibrumas kaj ekzercojn kaj lernantojn laŭtempe
- **Sen konto** — anonimaj uzantoj ricevas magiajn ligojn por konservi progreson; nedeviga pasŝlosila registrado por intersistema sinkronigo
- **Diversaj ekzercaj tipoj** — plenigi, plurelekto, lego, vortara karto (Anki-stile), bildo, aŭskultado, frazlibro
- **Komunumaj kontribuoj** — tradukoj, komentoj, eraraj raportoj, voĉdonado
- **Moderiga vico** — aŭtomata fido por establitaj uzantoj, mana revizio alie
- **Seria retumilo** — ekzercoj grupigitaj laŭ CEFR-nivelo (A0–C2) kaj temo
- **Lokaligita interfaco** — interfacaj ĉenoj en 30+ lingvoj; lernantoj vidas ekzercojn esperante sed difinoj en sia propra lingvo

## Teknika stako

- **Dorsa parto**: Go 1.22+ sur GAE-norma medio
- **Datumbazo**: Google Cloud Datastore
- **Antaŭa parto**: Go-ŝablonoj + HTMX (neniu JS-kadro)
- **CSS**: Pico CSS kun propraj ŝanĝoj
- **Aŭtentikigo**: magiaj ligiloj (crypto/rand) + WebAuthn-pasŝlosiloj

## Adapti por alia lingvo

Ĉi tiu kodo estas sufiĉe ĝenerala por kurso pri iu ajn lingvo. Por adapti:

1. Anstataŭigu semajn datumojn en `seed/` per ekzercoj por via cellingvo
2. Ĝisdatigu la lokalajn ĉenojn en `internal/locale/` (Esperantaj interfacaj ĉenoj estas en `eo.json`; elektu la plej proksiman ekzistantan lingvon aŭ aldonu novan)
3. Ĝisdatigu `app.yaml` per via propra GAE-projekta ID
4. Startu per `gcloud app deploy`

La nura Esperanto-specifa logiko estas en la semaj datumoj kaj lokalaj ĉenoj — la motoro mem estas lingvo-neŭtrala.

## Loka funkciado

```bash
# Postulas Google Cloud-projekton kun Datastore ebligita (aŭ uzu la emulatoron)
export GOOGLE_CLOUD_PROJECT=via-projekta-id
go run main.go
```

Per la Datastore-emulatorio:

```bash
gcloud beta emulators datastore start &
$(gcloud beta emulators datastore env-init)
go run main.go
```

## Permesilo

MIT — vidu [LICENSE](LICENSE).

Kontribuoj bonvenas. Se vi konstruas kurson por alia lingvo surbaze de ĉi tio, bonvolu sciigi nin!
