# influxdb-satellite-telemetry
Using IninfluxDB for storage of simulated satellite telemetry

# Why Use InfluxDB for Spacecraft Telemetry
If you’re not already familiar, InfluxDB is a time-series database from InfluxData that is much better than relational databases like Postgres, and document databases like MongoDB, at collecting time-series data

Telegraf is an agent platform for collecting time-series data and sending it to InfluxDB, which then stores the data for later analysis and presentation.

# Installation and Running
1. In a terminal, run ```influxd```
2. run ```source <(influx completion zsh)```
