# Desc-Format Monitor Lines — Design

**Status:** Draft
**Date:** 2026-04-17
**Issue:** [#75](https://github.com/erans/hyprmon/issues/75)

## Problem

Daisy-chained identical monitors (e.g., two displays over a single USB-C cable) receive connector names that the kernel assigns in a quasi-arbitrary order on boot or replug. For a user with DP-9 and DP-10, the pair frequently swaps, which reshuffles saved positions in `hyprland.conf`.

Hyprland supports matching monitors by EDID description instead of connector name:

```
monitor = desc:Dell Inc. DELL U3419W 5HJB6T2, 3440x1440@60, 0x0, 1
```

The description contains the serial number, so the match is stable across port swaps. HyprMon currently writes connector names only. This spec adds `desc:` format as an opt-in per-monitor setting.

## Non-goals

- Inferring `desc:` preference from existing lines in `hyprland.conf` (we do not parse user-authored config; the file is one-way output for this flag).
- Changing how live changes are applied via `hyprctl keyword monitor` (continues to use connector names).
- A bulk "enable desc: for all monitors" command. Per-monitor only.

## Data model

### `Monitor` struct (`models.go`)

Add one field:

```go
UseDescFormat bool `json:"use_desc_format,omitempty"`
```

Zero value is `false`; legacy profiles load unchanged.

### Settings file

New file: `~/.config/hyprmon/settings.json` (`$XDG_CONFIG_HOME/hyprmon/settings.json` when set; the existing `--cfg` flag points at the hyprmon config root, so the settings file follows it).

```json
{
  "monitor_prefs": {
    "Dell Inc./DELL U3419W/5HJB6T2": { "use_desc_format": true },
    "AOC/Q27V5N/FAAM2HA001234": { "use_desc_format": true }
  }
}
```

- Keyed by `HardwareID` (the existing stable `Make/Model/Serial` identifier).
- Extensible — other per-monitor preferences can join this map later without a new file.
- File mode `0600` (matches other hyprmon files). Atomic write via temp file + rename.

### Precedence layering

Lowest → highest; later layers overwrite earlier:

1. Hyprland live state (`readMonitors()`)
2. Settings file (per-HardwareID prefs, applied during `readMonitors()` post-processing)
3. Loaded profile JSON (when the user loads/applies a profile, profile values win)
4. Live UI edits

- Toggling in the advanced settings dialog updates the in-memory monitor **and** the settings file (the settings file holds the user's default across sessions).
- Loading/applying a profile overwrites the in-memory value; the settings file is **not** touched (the profile represents a one-shot override of the global default).
- Writing a profile serializes the current in-memory value into the profile JSON.
- Writing `hyprland.conf` reads the in-memory value.

## Write logic

### `generateMonitorLine(m Monitor)` — `hyprland.go`

Introduce an identifier selector at the top of the function:

```go
identifier := m.Name
if m.UseDescFormat && canUseDescFormat(m) {
    if desc := sanitizeDesc(m.EDIDName); desc != "" {
        identifier = "desc:" + desc
    }
}
```

All existing string-builder sites that use `m.Name` as the first token switch to `identifier`. This covers:

- Regular monitor line
- Mirror line (only the first token; the `,mirror,<SOURCE>` tail keeps the connector name of the source, because Hyprland's mirror target expects the connector name)
- Disable line (`monitor=desc:<desc>,disable` is valid Hyprland syntax)

### `canUseDescFormat(m Monitor) bool` — new helper

Returns `false` when any of:

- `m.EDIDName` is empty.
- Description is non-unique among currently-connected monitors. Detected by checking whether `disambiguateHardwareIDs` had to append `/#N` to this monitor's `HardwareID` — a `/#` suffix means duplicates exist and `desc:` cannot disambiguate them.
- Sanitization rejects the description (see below).

### `sanitizeDesc(s string) string` — new helper

- `TrimSpace`.
- Reject (return `""`) if the string contains `,` — Hyprland parses monitor lines on commas, so an embedded comma would break the line.
- Reject if the string contains `"` — we shell-quote lines in `applyMonitor`, and even though `applyMonitor` is unaffected by this feature today, the helper must not produce output a future caller could feed unsafely into a shell.
- Reject if the string contains control characters (`< 0x20`) or `\n`.

Rejection falls back to the connector name in `generateMonitorLine` (no error surfaced; the UI prevents the toggle from turning on for these cases, so rejection at write time is strictly defensive).

### `applyMonitor()` — unchanged

Keeps using `m.Name`. At apply time the connector name is guaranteed current; substring-matched `desc:` would add failure modes without benefit. Settled during brainstorming.

## Read logic

`readMonitors()` never parses desc-prefixed lines. After calling `hyprctl monitors -j` and building the monitor slice, it:

1. Calls `loadSettings()` (fresh read; cheap file).
2. For each monitor with non-empty `HardwareID`, looks up `monitor_prefs[hwid]` and applies `UseDescFormat` onto the in-memory monitor.

Monitors without a `HardwareID` never get the preference applied (same rule as other HardwareID-keyed logic).

## UI

### Advanced settings dialog (`advanced_settings.go`)

Add one row, "Write as desc:", alongside the existing rows (bit depth, color mode, VRR, transform).

Three visible states:

- `[ ] Write as desc:` — off, writes connector name. Default.
- `[x] Write as desc:` — on, writes `desc:<description>`.
- `[-] Write as desc: (unavailable — <reason>)` — disabled control. `<reason>` is one of:
  - "no EDID description"
  - "description not unique"
  - "description contains unsupported characters"

The control is disabled when `canUseDescFormat(m)` returns `false`. Toggling calls the new settings save helper.

### Profile details view (nice-to-have)

A small `d` glyph next to monitors that have `UseDescFormat = true` in the profile details listing. Not required; can land in a follow-up PR.

## Lifecycle details

- Startup: `loadSettings()` runs once; preferences merged into monitor list inside `readMonitors()`.
- Toggle flip in UI: update in-memory monitor, call `saveSettings()` (settings file written).
- Profile apply: profile values overwrite in-memory `UseDescFormat`. Settings file **not** modified.
- Profile save: current in-memory `UseDescFormat` is serialized into the profile JSON alongside other monitor fields.
- Exit: no on-exit write; all mutations are written synchronously.

## Testing

Unit tests in existing `_test.go` companions (new file if one does not exist for a given source file):

- `generateMonitorLine` — matrix:
  - desc off → connector line.
  - desc on, unambiguous description, no serial in description → desc line.
  - desc on, unambiguous description, with serial → desc line with serial.
  - desc on, ambiguous (duplicate hwid) → falls back to connector line.
  - desc on, empty `EDIDName` → falls back.
  - desc on, description contains comma → falls back.
  - desc on, disabled monitor → `monitor=desc:<desc>,disable`.
  - desc on, mirror monitor → `monitor=desc:<desc>,…,mirror,<SOURCE_CONNECTOR>`.
- `canUseDescFormat` — covers every rejection branch.
- `sanitizeDesc` — comma, quote, control character, whitespace-only inputs.
- Settings file round-trip: load → mutate → save → reload → values intact.
- Profile round-trip: marshal + unmarshal preserves `UseDescFormat`.

## Documentation

- `README.md`: a short section under monitor configuration explaining when to enable `desc:` format (daisy-chain, identical monitors with unique serials) and the unambiguous-description requirement.
- `CLAUDE.md`: extend "Monitor Data Flow" to note the per-monitor `UseDescFormat` preference and the settings file.

## File impact summary

| File | Change |
|---|---|
| `models.go` | Add `UseDescFormat bool` to `Monitor`. |
| `hyprland.go` | `generateMonitorLine` uses a new identifier selector; add `canUseDescFormat` and `sanitizeDesc`; `readMonitors` merges settings-file prefs after `disambiguateHardwareIDs`. |
| `settings.go` (new) | `loadSettings`, `saveSettings`, `getMonitorPref`, `setMonitorPref`. |
| `advanced_settings.go` | New row in dialog; disabled state with reason text. |
| `profiles.go` | No code change — `UseDescFormat` rides on the existing Monitor JSON. |
| `README.md` | Short section documenting the feature. |
| `CLAUDE.md` | Update Monitor Data Flow section. |
| `*_test.go` | Tests per the Testing section. |

## Open questions

None at time of writing.
