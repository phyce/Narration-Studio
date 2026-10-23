# Build Directory

Build files and assets for the app. The builds themselves are driven by the scripts in `scripts/` (see the main
README), not by `wails3 build`.

* `bin` - output directory
* `config.yml` - Wails v3 project info and the `wails3 dev` setup
* `appicon.png` - the app icon the platform icons were made from
* `darwin` - macOS files
* `windows` - Windows files

## Mac

- `Info.plist` - the app bundle's plist. `scripts/build-osx.sh` fills in `{{.Info.ProductVersion}}` from `wails.json`.
- `icons.icns` - the app icon, made with `wails3 generate icons -input build/appicon.png -macfilename build/darwin/icons.icns`.

## Windows

- `icon.ico` - the app icon.
- `wails.exe.manifest` - the app manifest. `scripts/build.ps1` fills in `{{.Info.ProductVersion}}` from `wails.json`
  and embeds it, the icon and the version info into the exe.
- `installer/*` - the NSIS installer (`project.nsi`, `wails_tools.nsh`) and the files it installs besides the app.
