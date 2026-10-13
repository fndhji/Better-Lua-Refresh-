# Extreme Lua Refresh

Hot-reload Lua addons in Garry's Mod without restarting the game after every change.

Extreme Lua Refresh (ELR) is a Windows development tool for people who spend more time editing Garry's Mod addons than restarting the game. Drop in a new Lua addon, create folders, change a script, or remove an addon while you're playing locally. ELR watches the files and reloads the affected Lua code.

It isn't a replacement for Garry's Mod's addon mounting system, and it won't magically make every third-party addon hot-reloadable. It's a practical way to shorten the edit-test-repeat loop.

## What it does

- Detects new Lua addons, files, and subfolders inside `garrysmod/addons/`.
- Reloads the changed addon instead of reloading everything.
- Handles server-side and client-side Lua, including common `autorun` layouts.
- Provides support for `include()` and file discovery in addons loaded from ELR snapshots.
- Tracks and cleans up supported hooks, timers, console commands, and other registrations during reloads.
- Performs syntax checks before applying an update. Runtime rollback is limited.
- Includes experimental loading for Lua SWEPs and scripted entities.
- Runs through a small desktop installer with a live log, English/French toggle, and optional automatic monitoring.

**Current versions:** Desktop installer **v1.0.1** / Lua refresh engine **v0.1.1**.

## Requirements

- Windows 10 or 11, 64-bit
- Garry's Mod installed through Steam
- Windows PowerShell 5.1 or newer
- A local Garry's Mod session for the currently tested workflow

No DLL injection, kernel driver, or extra game modification is required. The desktop application is built with Go and uses the Windows APIs; the file watcher uses PowerShell.

## Install

1. Download `ExtremeLuaRefresh_Installer_v1_0_1.exe` from the project's release files.
2. Run it and check the detected Garry's Mod path. If it isn't right, select your `garrysmod` folder manually.
3. Click **Install / Update**.
4. Start Garry's Mod. If this is your first installation and GMod was already running, restart the game **once** so it can load ELR's Lua bootstrap.
5. Click **Start** to run the monitor manually, or enable **Auto-start**.

A typical installation folder looks like this:

```text
C:\Program Files (x86)\Steam\steamapps\common\GarrysMod\garrysmod\
└── addons\
    └── extreme_lua_refresh\
        ├── lua\
        │   ├── autorun\
        │   │   ├── client\
        │   │   └── server\
        │   └── elr\
        └── tools\
            └── Watcher.ps1
```

You can also select the main `GarrysMod` directory or its `addons` directory; the installer resolves the game folder automatically.

### Everyday workflow

Once ELR is installed and its monitor is running:

1. Keep your local GMod session open.
2. Add an addon folder under `garrysmod/addons/`, or edit a Lua file in an existing addon.
3. Save the file.
4. Check the game's console or the desktop application's log if the change doesn't appear.

For example, create `garrysmod/addons/hello_elr/lua/autorun/hello_elr.lua`:

```lua
print("[ELR] Hello from a newly added addon")
```

Save it while GMod is running. ELR should pick it up without another game restart. Change the message and save again to test a reload.

## Desktop controls

| Control | Purpose |
| --- | --- |
| **Install / Update** | Installs or replaces the bundled Lua refresh files. |
| **Start** | Starts the background monitor. |
| **Stop** | Stops the monitor and its watcher. |
| **Auto-start** | Registers a small background agent at Windows sign-in. |
| **English / Français** | Changes the app language. |
| **Guide / Tutorial** | Shows the built-in quick-start instructions. |
| **Live Console** | Displays installer, monitor, and watcher logs. |

### How auto-start works

Auto-start is **optional**. When enabled, ELR registers its background agent for your Windows user account. The agent waits for Garry's Mod to launch, starts the PowerShell watcher when the game is detected, and stops the watcher when the game closes.

Closing the desktop window does **not** stop this agent. It stays idle so it can detect your next GMod session. Turning Auto-start off removes the Windows startup entry and stops the agent.

This uses the current user's Windows startup registry entry (`HKCU`), not a system-wide service. You can still use **Start** and **Stop** manually without enabling auto-start.

## How the refresh works

ELR has three moving parts:

```text
File change in garrysmod/addons/
             |
             v
    PowerShell watcher
    detects & groups edits
             |
             v
   Lua snapshot + manifest
     in garrysmod/data/
             |
             v
    GMod Lua refresh engine
       server / client
             |
             v
        Updated addon
```

The watcher observes file and folder changes, waits briefly for a burst of edits to settle, and writes a snapshot of the affected addon's Lua files. The in-game Lua component reads that snapshot and loads the relevant code. For client-side updates, the server sends the needed Lua data to the client.

ELR can work with newly created Lua folders even when the Source engine hasn't mounted that addon in the normal way. Some file APIs are handled by ELR for this purpose. Native file access and other engine resources are **not** fully virtualized.

The watcher currently groups changes after about **240 ms** of inactivity. Actual time to see an update includes file detection, snapshot generation, in-game polling, transfer, and execution. It varies with addon size and system load.

## In-game commands

Run these from the Garry's Mod console in the supported local-host workflow:

```text
elr_status
elr_client_status
elr_force hello_elr
```

- `elr_status` reports server-side managed addons.
- `elr_client_status` shows client-side loaded addons.
- `elr_force <addon_folder>` requests a fresh snapshot of that addon.

Forced refresh access is restricted to the local host and authorized administrators.

## Test results

ELR has been tested in a local Garry's Mod session using generated test addons. The tests covered:

| Scenario | Observed result |
| --- | --- |
| Add, edit, and remove small Lua addons | Passed in the test setup |
| Add Lua files and nested modules while running | Passed |
| Server/client refresh and basic networking | Passed |
| Experimental SWEP and scripted entity loading | Passed with test objects |
| Load 1,000 Lua modules | 1,000 / 1,000 loaded |
| Modify 200 modules in one batch | 200 / 200 detected |
| 120 successive reload cycles | Passed the test checks |
| Introduce a Lua syntax error | Update rejected; previous version retained |

In the 1,000-module test, the *Lua execution step* generally took roughly **14–22 ms on the server** and **19–29 ms on the client**. These figures are **not** the total save-to-screen refresh time, and these synthetic tests don't guarantee compatibility with a large production addon.

## Known limitations

- **Lua only.** New models, materials, maps, sounds, VPKs, GMA files, fonts, HTML assets, and native DLL modules are not mounted automatically by ELR.
- **4 MB per addon.** This version rejects snapshots containing more than 4 MB of Lua source for a single addon.
- **Not universal.** Addons that rely heavily on engine mounting, native `require()` modules, unusual load orders, or stateful systems may need manual cleanup or a full restart.
- **Cleanup is best-effort.** ELR tracks supported registrations, but cannot reliably undo every side effect produced by arbitrary Lua code.
- **Experimental SWEP/entity support.** Loading a test SWEP or entity is not the same as guaranteeing that every existing instance can be safely replaced.
- **Potential double refresh.** Garry's Mod's built-in Lua refresh may also react to changes in already mounted addons. If you run into this, test with a newly added addon first. The game's `-disableluarefresh` launch option may help, but changing launch options requires restarting the game.

ELR is currently a development tool, not a promise of zero errors or production-safe updates for every addon.

## Troubleshooting

**The installer freezes or shows “Not Responding”**

Use Desktop v1.0.1 or later. Check the startup log at:

```text
%LOCALAPPDATA%\ExtremeLuaRefresh\app.log
```

**The app says it can't find Garry's Mod**

Choose the `garrysmod` folder containing `addons`. Installations in another Steam library may need manual selection.

**Nothing reloads**

Confirm that ELR was installed before the current GMod session started, that the monitor is running, and that you're editing files inside a folder under `garrysmod/addons/`. Try `elr_status`, then `elr_force addon_folder`.

**The log shows duplicate refreshes**

Close any old `DEMARRER_REFRESH.bat` or `Watcher.ps1` process. Run only one ELR monitor. Also check whether GMod's built-in refresh is acting on the same files.

**An addon is refused for being too large**

The current snapshot limit is 4 MB of Lua per addon. Splitting the addon into smaller independent components may help during development, but it isn't a universal fix for cross-addon dependencies.

**Where are the logs?**

```text
%LOCALAPPDATA%\ExtremeLuaRefresh\app.log
%LOCALAPPDATA%\ExtremeLuaRefresh\watcher.log
```

The desktop app also reads these logs into its live console.

## Build from source

The Windows desktop installer is written in Go 1.23+ with standard-library and Win32 API calls. The repo needs the bundled `payload.zip`, which contains the Lua loader and watcher.

From the desktop source directory on Windows:

```powershell
go test ./...
go build -trimpath -ldflags="-H windowsgui -s -w" -o ExtremeLuaRefresh_Installer.exe .
```

The included `payload.zip` is embedded at build time. A Go build alone does not produce a new Lua engine version: update the payload separately if you change its scripts.

## Remove ELR

1. Turn **Auto-start** off and press **Stop** in the desktop app.
2. Close any manual watcher still running.
3. Delete `garrysmod/addons/extreme_lua_refresh`.
4. Restart GMod to unload its Lua bootstrap completely.
5. If you also want to delete settings and logs, remove `%LOCALAPPDATA%\ExtremeLuaRefresh`.

ELR's in-game snapshots are stored under `garrysmod/data/extreme_refresh/` and can be removed with GMod closed.

---

Built for a simpler workflow: edit Lua, save, test, repeat. No game restart unless the engine actually needs one.
