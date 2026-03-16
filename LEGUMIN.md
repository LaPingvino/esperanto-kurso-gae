# Esperanto-kurso

Interaga lingvolerna platformo por Esperanto, atingebla ĉe **[esperanto-kurso.net](https://esperanto-kurso.net)**.

## Kio ĝi estas

Mem-gastigata, adaptiĝema lernplatformo konstruita per Go sur Google App Engine. Ĝiaj trajtoj:

- **Adaptiĝema malfacileco** — Glicko-2 taksoj kalibrumas kaj ekzercojn kaj lernantojn laŭtempe
- **Sen konto** — anonimaj uzantoj ricevas magiajn ligilojn por konservi progreson; nedeviga registrado per ensalutŝlosilo por sinkronigo inter aparatoj
- **Diversaj ekzercaj tipoj** — plenigi, plurelekto, legado, vortara karto (Anki-stile), bildo, aŭskultado, frazaro
- **Komunumaj kontribuoj** — tradukoj, komentoj, erarraporto, voĉdonado
- **Moderiga vico** — aŭtomata fido por establitaj uzantoj, mana revizio aliokaze
- **Seria retumilo** — ekzercoj grupigitaj laŭ CEFR-nivelo (A0–C2) kaj temo
- **Plurlingva interfaco** — interfacaj tekstoj en 30+ lingvoj; lernantoj vidas ekzercojn esperante, sed difinoj aperas en ilia propra lingvo

## Teknika stako

- **Servila flanko**: Go 1.22+ sur GAE-norma medio
- **Datumbazo**: Google Cloud Datastore
- **Klienta flanko**: Go-ŝablonoj + HTMX (neniu JS-kadro)
- **CSS**: Pico CSS kun propraj ŝanĝoj
- **Aŭtentikigo**: magiaj ligiloj (crypto/rand) + WebAuthn-ensalutŝlosiloj

## Adapti por alia lingvo

Ĉi tiu kodo estas sufiĉe ĝenerala por kurso pri iu ajn lingvo. Por adapti:

1. Anstataŭigu la semajn datumojn en `seed/` per ekzercoj por via cellingvo
2. Ĝisdatigu la lokajn tekstojn en `internal/locale/` (esperantaj tekstoj estas en `eo.json`; elektu la plej proksiman ekzistantan lingvon aŭ aldonu novan)
3. Ĝisdatigu `app.yaml` per via propra GAE-projekta ID
4. Rulu `gcloud app deploy`

La nura Esperanto-specifa logiko estas en la semaj datumoj kaj lokaj tekstoj — la motoro mem estas lingve neŭtrala.

## Loka ekfunkciigo

```bash
# Postulas Google Cloud-projekton kun ebligita Datastore (aŭ uzu la emulatoron)
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
