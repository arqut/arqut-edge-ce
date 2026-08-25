# Changelog
All notable changes to this project will be documented in this file.

## [0.6.0] - 2026-08-25
### Features
- configure Home Assistant trusted proxies over the websocket API on Home Assistant 2026.8 and later, where the `http` integration moved out of `configuration.yaml` into Settings > System > Network. Applying a change there restarts Home Assistant and has to be confirmed afterwards, both of which are handled; earlier versions keep using `configuration.yaml`
- added `haaddon.HTTPConfigMode`, reporting which of the two mechanisms an instance uses so callers can describe it without parsing versions

### Fixes
- reconcile Home Assistant trusted proxies on every start rather than only on the one that creates the tunnel, so a setting that was edited away or reverted is put back instead of staying lost until the tunnel is recreated
- exposing Home Assistant no longer refuses when the tunnel is already there. An existing tunnel says nothing about the trusted proxy setting, which the call re-applies

### Changed
- `haaddon.UpdateHAConfig` now takes a `context.Context` (breaking for callers). It blocks while Home Assistant restarts and confirms the change, so it should be run in the background
- exposing Home Assistant keeps the settings page in view instead of navigating to the services list, now that the button doubles as a repair for the trusted proxy setting

## [0.5.1] - 2026-07-20
### Fixes
- restart a running proxy service automatically when its local host, port, or enabled state is changed via the update API, instead of requiring a manual toggle

## [0.5.0] - 2026-07-03
### Features
- added option to require authentication for services
- added retry logic for proxy service binding when TUN device is unavailable

## [0.4.0] - 2026-05-26
### Features
- added service list pagination
- added connect/disconnect events for wireguard service
- added turn creds getter method

## [0.3.0] - 2026-03-30
### Features
- supported to serve local secure service HTTPs

## [0.2.0] - 2026-02-04
### Features
- added path to service

## [0.1.0] - 2025-11-19
### Added
- Initial release
