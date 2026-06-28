# oe-module-hl7-hub

OpenEMR adapter for the EMR-agnostic [HL7 hub](https://github.com/rollingventures/hl7-hub).

This module is the **thin, OpenEMR-specific half** of the integration. It does
not speak HL7 — it speaks the hub's neutral *canonical* model. All HL7 v2
encoding/decoding, MLLP transport, routing and audit live in the hub. That
separation is what lets the same hub serve any EMR; OpenEMR is adapter #1.

## What it does (Milestone 1 — ADT)

- **Outbound:** subscribes to `patient.created` / `patient.updated`
  (`PatientEventSubscriber`), maps the `patient_data` row to canonical
  (`Canonical\PatientMapper`), and POSTs it to the hub's `POST /events`
  (`Client\HubClient`). A hub outage is logged, never blocks the clinical save.
- **Inbound:** `public/webhook.php` accepts a signed canonical patient from the
  hub and writes it via `PatientService` (create in M1).

Both directions are authenticated with a shared secret (`X-Hub-Secret`,
constant-time compared).

## Configuration (M1)

Set environment variables visible to the OpenEMR PHP process:

| Var | Meaning |
|-----|---------|
| `HL7HUB_URL` | hub control-plane base URL, e.g. `http://hub:8088` |
| `HL7HUB_SECRET` | shared secret, must match the hub's `-secret` / `HUB_SECRET` |

If `HL7HUB_URL` is unset the module registers no listeners (inert). A later
increment moves these to OpenEMR globals via `GlobalsInitializedEvent` and adds
an admin settings panel + menu entry.

## Layout

```
openemr.bootstrap.php          module entry; registers PSR-4 + Bootstrap
src/Bootstrap.php              subscribes the patient event listener
src/GlobalConfig.php           hub URL + shared secret
src/Subscriber/PatientEventSubscriber.php   patient.created/updated -> hub
src/Canonical/PatientMapper.php             patient_data <-> canonical (the only EMR-aware code)
src/Client/HubClient.php       signed POST to hub /events
public/webhook.php             signed inbound: canonical -> PatientService
```

## Status

Milestone-1 scaffold. **Not yet verified against a running stack** — install via
the Module Manager, set the env vars, and exercise create/update. Known
follow-ups: upsert-by-MRN on inbound, admin settings UI (globals), encounter /
appointment (SIU) feeds, and the lab ORM/ORU flow (M2).
