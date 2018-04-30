# nutclient

nutclient is a minimal client which interacts with a NUT (Network UPS Tools) server.

It was written because my Zyxel NSA320 NAS only came with apcupsd and not with nut.

It doesn't have any fancy timers like the official nut-client. Instead, it calls `SHUTDOWNCMD` as soon as the UPS switches to battery power.