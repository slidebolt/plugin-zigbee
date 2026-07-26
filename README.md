# Slidebolt Zigbee Plugin

This Go plugin mirrors a standalone `zigbee-host` into Slidebolt. It never
opens the serial adapter and stores no Zigbee network credentials.

Configure `ZIGBEE_HOST_URL` and `ZIGBEE_HOST_TOKEN`. The plugin fetches one
snapshot, follows replayable SSE changes, and reconnects without requiring a
specific process startup order.
