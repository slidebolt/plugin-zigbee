# Slidebolt Zigbee Plugin

This Go plugin mirrors a standalone `zigbee-host` into Slidebolt. It never
opens the serial adapter and stores no Zigbee network credentials.

Configure `ZIGBEE_HOST_URL` and `ZIGBEE_HOST_TOKEN`. The plugin fetches one
snapshot, follows replayable SSE changes, and reconnects without requiring a
specific process startup order.

Zigbee-specific evidence stays behind this plugin boundary. Every physical
node is published as one ordinary Slidebolt device with generic connectivity,
signal-quality, and network-link entities. The latter is a `diagnostic` entity
containing address history, parent device key, relationship/depth, inbound and
outbound timestamps, LQI, and the last failure. Device and entity status use
the shared `healthy`, `degraded`, `unavailable`, and `unknown` health values.

The coordinator exposes the shared `button_press` command as “Refresh Network
Diagnostics.” The plugin privately translates that generic command to the
host's refresh operation; core APIs and UIs require no Zigbee special case.
